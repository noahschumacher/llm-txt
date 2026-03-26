package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockCompleter is a test double for llm.Completer.
type mockCompleter struct {
	responses []string
	err       error
	calls     int
}

func (m *mockCompleter) Complete(_ context.Context, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.calls >= len(m.responses) {
		return "Tie\nbecause.", nil
	}
	r := m.responses[m.calls]
	m.calls++
	return r, nil
}

// ── fetchGroundTruth ──────────────────────────────────────────────────────────

func TestFetchGroundTruth(t *testing.T) {
	const body = "# example.com\n\n> Root.\n\n## Docs\n\n- [Page](https://example.com/page): A page.\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	d, err := fetchGroundTruth(context.Background(), srv.URL+"/llms.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.host != "example.com" {
		t.Errorf("host = %q, want %q", d.host, "example.com")
	}
	if len(d.entries) != 1 {
		t.Errorf("entries = %d, want 1", len(d.entries))
	}
}

func TestFetchGroundTruth_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchGroundTruth(context.Background(), srv.URL+"/llms.txt")
	if err == nil {
		t.Error("expected error for non-200 response, got nil")
	}
}

// ── compareGroundTruth ────────────────────────────────────────────────────────

func TestCompareGroundTruth_URLCoverage(t *testing.T) {
	ours := doc{
		entries: []evalEntry{
			{url: "https://example.com/a", desc: "Our A."},
			{url: "https://example.com/b", desc: "Our B."},
		},
	}
	gt := doc{
		entries: []evalEntry{
			{url: "https://example.com/a", desc: "GT A."},
			{url: "https://example.com/b", desc: "GT B."},
			{url: "https://example.com/c", desc: "GT C."}, // not in ours
		},
	}
	pageBodies := map[string]string{
		"https://example.com/a": "body a",
		"https://example.com/b": "body b",
	}

	r := compareGroundTruth(ours, gt, pageBodies)

	// 2 of 3 GT URLs are covered
	want := 2.0 / 3.0
	if r.urlCoverage < want-0.01 || r.urlCoverage > want+0.01 {
		t.Errorf("urlCoverage = %.4f, want %.4f", r.urlCoverage, want)
	}
}

func TestCompareGroundTruth_FalsePositives(t *testing.T) {
	ours := doc{
		entries: []evalEntry{
			{url: "https://example.com/a", desc: "Our A."},
			{url: "https://example.com/extra", desc: "Not in GT."},
		},
	}
	gt := doc{
		entries: []evalEntry{
			{url: "https://example.com/a", desc: "GT A."},
		},
	}

	r := compareGroundTruth(ours, gt, nil)

	if r.falsePositives != 1 {
		t.Errorf("falsePositives = %d, want 1", r.falsePositives)
	}
}

func TestCompareGroundTruth_SectionAlignment(t *testing.T) {
	ours := doc{sections: []string{"Docs", "Blog", "Extra"}}
	gt := doc{
		sections: []string{"Docs", "Blog", "About"},
		entries:  []evalEntry{},
	}

	r := compareGroundTruth(ours, gt, nil)

	// 2 of 3 GT sections match (Docs, Blog)
	want := 2.0 / 3.0
	if r.sectionAlignment < want-0.01 || r.sectionAlignment > want+0.01 {
		t.Errorf("sectionAlignment = %.4f, want %.4f", r.sectionAlignment, want)
	}
}

func TestCompareGroundTruth_OverlapBuilding(t *testing.T) {
	ours := doc{
		entries: []evalEntry{
			{url: "https://example.com/a", desc: "Our A."},
			{url: "https://example.com/b", desc: ""},          // blank — excluded from overlap
			{url: "https://example.com/c", desc: "Our C."},
		},
	}
	gt := doc{
		entries: []evalEntry{
			{url: "https://example.com/a", desc: "GT A."},
			{url: "https://example.com/b", desc: "GT B."},
			{url: "https://example.com/c", desc: ""},          // blank GT desc — excluded from overlap
		},
	}
	pageBodies := map[string]string{
		"https://example.com/a": "body a",
		"https://example.com/b": "body b",
		"https://example.com/c": "body c",
	}

	r := compareGroundTruth(ours, gt, pageBodies)

	// only /a qualifies: both sides have non-blank descs and there's a body
	if len(r.overlap) != 1 {
		t.Errorf("overlap = %d, want 1", len(r.overlap))
	}
	if len(r.overlap) > 0 && r.overlap[0].url != "https://example.com/a" {
		t.Errorf("overlap[0].url = %q, want /a", r.overlap[0].url)
	}
}

func TestCompareGroundTruth_EmptyGT(t *testing.T) {
	ours := doc{entries: []evalEntry{{url: "https://example.com/a", desc: "A."}}}
	r := compareGroundTruth(ours, doc{}, nil)
	if r.urlCoverage != 0 {
		t.Errorf("urlCoverage with empty GT = %.2f, want 0", r.urlCoverage)
	}
}

// ── runJudge ──────────────────────────────────────────────────────────────────

func TestRunJudge_Verdicts(t *testing.T) {
	overlap := []overlapEntry{
		{url: "https://example.com/a", body: "body", ourDesc: "Ours A.", gtDesc: "GT A."},
		{url: "https://example.com/b", body: "body", ourDesc: "Ours B.", gtDesc: "GT B."},
		{url: "https://example.com/c", body: "body", ourDesc: "Ours C.", gtDesc: "GT C."},
		{url: "https://example.com/d", body: "body", ourDesc: "Ours D.", gtDesc: "GT D."},
	}
	mock := &mockCompleter{
		responses: []string{
			"A\nbecause ours is better.",
			"B\nbecause GT is better.",
			"Tie\nbecause they're equal.",
			"A\nbecause ours is better.",
		},
	}

	r := runJudge(context.Background(), mock, overlap, 10)

	if r.total != 4 {
		t.Errorf("total = %d, want 4", r.total)
	}
	if r.ourWins != 2 {
		t.Errorf("ourWins = %d, want 2", r.ourWins)
	}
	if r.gtWins != 1 {
		t.Errorf("gtWins = %d, want 1", r.gtWins)
	}
	if r.ties != 1 {
		t.Errorf("ties = %d, want 1", r.ties)
	}
}

func TestRunJudge_MaxSamples(t *testing.T) {
	overlap := make([]overlapEntry, 10)
	for i := range overlap {
		overlap[i] = overlapEntry{body: "b", ourDesc: "o", gtDesc: "g"}
	}
	mock := &mockCompleter{responses: []string{}}

	r := runJudge(context.Background(), mock, overlap, 3)

	if r.total != 3 {
		t.Errorf("total = %d, want 3 (maxSamples respected)", r.total)
	}
}

func TestRunJudge_ErrorCountedAsTie(t *testing.T) {
	overlap := []overlapEntry{
		{body: "b", ourDesc: "o", gtDesc: "g"},
		{body: "b", ourDesc: "o", gtDesc: "g"},
	}
	mock := &mockCompleter{err: errors.New("api error")}

	r := runJudge(context.Background(), mock, overlap, 10)

	if r.ties != 2 {
		t.Errorf("ties = %d, want 2 (errors count as ties)", r.ties)
	}
	if r.ourWins != 0 || r.gtWins != 0 {
		t.Errorf("expected no wins on error, got ourWins=%d gtWins=%d", r.ourWins, r.gtWins)
	}
}
