package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type generateRequest struct {
	URL      string `json:"url"`
	Mode     string `json:"mode"` // "basic" | "enhanced"
	FullText bool   `json:"full_text"`
}

type sseEvent struct {
	Type string `json:"type"` // "progress" | "done" | "error"

	// progress
	Message string `json:"message,omitempty"`

	// done
	LLMsTxt     string `json:"llms_txt,omitempty"`
	LLMsFullTxt string `json:"llms_full_txt,omitempty"`
	Summary     string `json:"summary,omitempty"`
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
	var req generateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	if _, err := url.ParseRequestURI(req.URL); err != nil {
		http.Error(w, "invalid url", http.StatusBadRequest)
		return
	}
	if req.Mode != "basic" && req.Mode != "enhanced" {
		req.Mode = "basic"
	}

	sse, ok := newSSEWriter(w)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	s.log.Sugar().Infof("generate request: url=%s mode=%s full_text=%v", req.URL, req.Mode, req.FullText)

	// TODO: replace with real crawler + generator pipeline
	steps := []string{
		"Validating URL...",
		"Fetching robots.txt...",
		"Discovering sitemap...",
		"Crawling pages...",
		"Inferring sections...",
	}
	for _, step := range steps {
		sse.progress(step)
		time.Sleep(time.Second)
	}

	sse.send(sseEvent{
		Type: "done",
		LLMsTxt: `# Example Site

> A platform for developers to discover and share code snippets, tools, and resources.

## Docs

- [Getting Started](https://example.com/docs/getting-started): Introduction to the platform, account setup, and first steps.
- [API Reference](https://example.com/docs/api): Full reference for the REST API including authentication and endpoints.
- [SDKs](https://example.com/docs/sdks): Official client libraries for Python, Go, TypeScript, and Ruby.

## Blog

- [What's New in v3](https://example.com/blog/whats-new-v3): Overview of the major features and breaking changes in the v3 release.
- [Building with the API](https://example.com/blog/building-with-api): A walkthrough of a real integration built on top of the public API.

## About

- [About Us](https://example.com/about): Mission, team, and company background.
- [Pricing](https://example.com/pricing): Plan comparison and pricing details for individuals and teams.
`,
		Summary: "7 pages crawled · 3 sections · mode: " + req.Mode,
	})
}
