package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"go.uber.org/zap"

	"github.com/noahschumacher/llm-txt/crawler"
)

type generateRequest struct {
	URL      string `json:"url"`
	Mode     string `json:"mode"`       // "basic" | "enhanced"
	MaxPages int    `json:"max_pages"`
	MaxDepth int    `json:"max_depth"`
}

type sseEvent struct {
	Type string `json:"type"` // "progress" | "done" | "error"

	// progress
	Message string `json:"message,omitempty"`
	Crawled int    `json:"crawled,omitempty"` // pages fetched so far
	Total   int    `json:"total,omitempty"`   // max pages (upper bound)

	// done
	LLMsTxt string `json:"llms_txt,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// sseWriter sends Server-Sent Events over an open HTTP connection.
// Each event is written as "data: <json>\n\n" and flushed immediately
// so the client receives it without waiting for the handler to return.
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func newSSEWriter(w http.ResponseWriter) (*sseWriter, bool) {
	// http.Flusher lets us push bytes to the client mid-handler.
	// Not all ResponseWriter implementations support it (e.g. test recorders).
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // prevent nginx from buffering the stream
	return &sseWriter{w: w, flusher: f}, true
}

func (s *sseWriter) send(ev sseEvent) error {
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", b); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func (s *sseWriter) progress(msg string) {
	s.send(sseEvent{Type: "progress", Message: msg})
}

func (s *sseWriter) error(msg string) {
	s.send(sseEvent{Type: "error", Message: msg})
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	req, err := parseGenerateRequest(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sse, ok := newSSEWriter(w)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	s.log.Info("generate request", zap.String("url", req.URL), zap.String("mode", req.Mode), zap.Int("max_pages", req.MaxPages), zap.Int("max_depth", req.MaxDepth))

	cfg := s.cfg.CrawlConfig
	if req.MaxPages > 0 {
		cfg.MaxPages = req.MaxPages
	}
	if req.MaxDepth > 0 {
		cfg.MaxDepth = req.MaxDepth
	}
	maxPages := cfg.MaxPages
	if maxPages == 0 {
		maxPages = 50
	}

	c := crawler.New(cfg, s.log)
	c.OnPage = func(crawled int) {
		sse.send(sseEvent{
			Type:    "progress",
			Message: "Crawling...",
			Crawled: crawled,
			Total:   maxPages,
		})
	}

	sse.progress("Fetching robots.txt and sitemap...")
	pages, err := c.Crawl(r.Context(), req.URL)
	if err != nil && len(pages) == 0 {
		s.log.Error("crawl failed", zap.Error(err))
		sse.error("crawl failed: " + err.Error())
		return
	}
	sse.progress(fmt.Sprintf("Crawled %d pages. Formatting output...", len(pages)))

	llmsTxt := formatLLMsTxt(req.URL, pages)
	sse.send(sseEvent{
		Type:    "done",
		LLMsTxt: llmsTxt,
		Summary: fmt.Sprintf("%d pages crawled · mode: %s", len(pages), req.Mode),
	})
}

// parseGenerateRequest decodes and validates the request body. Mode defaults to
// "basic" if omitted or unrecognized. The URL is normalized to its origin
// (scheme + host) — paths and query params are stripped since llms.txt
// represents the whole site.
func parseGenerateRequest(body io.Reader) (generateRequest, error) {
	var req generateRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return req, errors.New("invalid request body")
	}
	if req.URL == "" {
		return req, errors.New("url is required")
	}

	normalized, err := normalizeURL(req.URL)
	if err != nil {
		return req, errors.New("invalid url")
	}
	req.URL = normalized

	if req.Mode != "basic" && req.Mode != "enhanced" {
		req.Mode = "basic"
	}
	return req, nil
}

// formatLLMsTxt assembles a basic llms.txt from crawled pages.
// Pages are grouped into sections by their first URL path segment
// (e.g. /docs/* → "Docs"). Pages at the root go into a "General" section.
func formatLLMsTxt(origin string, pages []crawler.Page) string {
	// Find the root page description to use as the site-level blockquote
	// and as a filter — pages that share it have no unique description.
	var rootDesc string
	for _, p := range pages {
		if p.URL == origin || p.URL == origin+"/" {
			rootDesc = p.Description
			break
		}
	}

	// Collect section names in insertion order, skipping pages that add no
	// unique value (same description as root = inherited global meta tag).
	order := make([]string, 0, len(pages))
	sections := make(map[string][]crawler.Page, len(pages))

	for _, p := range pages {
		isRoot := p.URL == origin || p.URL == origin+"/"
		if isRoot {
			continue // root is already represented by the H1 + blockquote
		}
		if p.Description == rootDesc {
			continue // inherited global meta, no unique content
		}
		sec := sectionName(p.URL, origin)
		if _, exists := sections[sec]; !exists {
			order = append(order, sec)
		}
		sections[sec] = append(sections[sec], p)
	}

	host := strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", host)

	if rootDesc != "" {
		fmt.Fprintf(&b, "> %s\n\n", firstSentence(rootDesc))
	}

	for _, sec := range order {
		fmt.Fprintf(&b, "## %s\n\n", sec)
		for _, p := range sections[sec] {
			title := p.Title
			if title == "" {
				title = p.URL
			}
			desc := p.Description
			if desc == "" {
				desc = firstSentence(p.Body)
			}
			if desc != "" {
				fmt.Fprintf(&b, "- [%s](%s): %s\n", title, p.URL, desc)
			} else {
				fmt.Fprintf(&b, "- [%s](%s)\n", title, p.URL)
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// sectionName infers a display section from the first path segment of u.
func sectionName(u, origin string) string {
	path := strings.TrimPrefix(u, origin)
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "General"
	}
	seg := strings.SplitN(path, "/", 2)[0]
	// Strip query strings from the segment.
	seg = strings.SplitN(seg, "?", 2)[0]
	if seg == "" {
		return "General"
	}
	return strings.ToUpper(seg[:1]) + seg[1:]
}

// firstSentence returns the first sentence of text (up to 160 chars).
func firstSentence(text string) string {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return ""
	}
	for i, ch := range text {
		if (ch == '.' || ch == '!' || ch == '?') && i > 0 {
			return text[:i+1]
		}
	}
	if len(text) > 160 {
		return text[:160] + "..."
	}
	return text
}

// normalizeURL ensures the URL has a scheme and strips path/query/fragment
// so the crawler always starts from the site root.
func normalizeURL(raw string) (string, error) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host, nil
}
