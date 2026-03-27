package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/noahschumacher/llm-txt/crawler"
	"github.com/noahschumacher/llm-txt/services/generator"
)

type generateRequest struct {
	URL         string `json:"url"`
	Mode        string `json:"mode"` // "basic" | "enhanced"
	MaxPages    int    `json:"max_pages"`
	MaxDepth    int    `json:"max_depth"`
	Concurrency int    `json:"concurrency"` // 1=standard, 3=fast, 5=turbo
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
	if r.Header.Get("X-Password") != s.cfg.Password {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

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

	s.log.Info(
		"generate request",
		zap.String("url", req.URL),
		zap.String("mode", req.Mode),
		zap.Int("max_pages", req.MaxPages),
		zap.Int("max_depth", req.MaxDepth),
		zap.Int("concurrency", req.Concurrency),
	)

	cfg := s.cfg.CrawlConfig
	if req.MaxPages > 0 {
		cfg.MaxPages = req.MaxPages
	}
	if req.MaxDepth > 0 {
		cfg.MaxDepth = req.MaxDepth
	}
	if req.Concurrency > 0 {
		cfg.Concurrency = req.Concurrency
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
	sse.progress(fmt.Sprintf("Crawled %d pages. Generating descriptions...", len(pages)))

	result := s.generator.Generate(r.Context(), pages, generator.Options{
		Mode:   req.Mode,
		Origin: req.URL,
		OnDescribe: func(completed int) {
			sse.send(sseEvent{
				Type:    "progress",
				Message: "Generating descriptions...",
				Crawled: completed,
				Total:   len(pages),
			})
		},
	})
	sse.send(sseEvent{
		Type:    "done",
		LLMsTxt: result.LLMsTxt,
		Summary: result.Summary,
	})

	if s.cfg.AppEnv == "local" {
		go s.writeDebugOutputs(req.URL, req.Mode, pages, result)
	}
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

// writeDebugOutputs writes the requested mode's output alongside a basic
// comparison to os.TempDir()/llm-txt/ for local inspection.
func (s *Server) writeDebugOutputs(origin, mode string, pages []crawler.Page, result generator.Result) {
	dir := filepath.Join(os.TempDir(), "llm-txt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.log.Debug("debug output: could not create dir", zap.Error(err))
		return
	}

	host := strings.NewReplacer("https://", "", "http://", "", "/", "", ".", "-").Replace(origin)
	ts := time.Now().Format("20060102-150405")
	prefix := filepath.Join(dir, fmt.Sprintf("%s-%s", ts, host))

	write := func(path, content string) {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			s.log.Debug("debug output: write failed", zap.String("path", path), zap.Error(err))
		}
	}

	write(prefix+"-"+mode+".txt", result.LLMsTxt)

	// always write a basic version for comparison when enhanced was requested
	if mode == "enhanced" {
		basic := s.generator.Generate(context.Background(), pages, generator.Options{
			Mode:   "basic",
			Origin: origin,
		})
		write(prefix+"-basic.txt", basic.LLMsTxt)
	}

	s.log.Debug("debug outputs written", zap.String("dir", dir), zap.String("prefix", filepath.Base(prefix)))
}
