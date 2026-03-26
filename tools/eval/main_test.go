package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// ── normalizeOrigin ───────────────────────────────────────────────────────────

func TestNormalizeOrigin(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"full https URL", "https://example.com", "https://example.com", false},
		{"full http URL", "http://example.com", "http://example.com", false},
		{"no scheme — adds https", "example.com", "https://example.com", false},
		{"strips path", "https://example.com/docs/page", "https://example.com", false},
		{"strips query", "https://example.com?foo=bar", "https://example.com", false},
		{"strips trailing slash", "https://example.com/", "https://example.com", false},
		{"subdomain preserved", "https://docs.example.com/page", "https://docs.example.com", false},
		{"invalid URL", "://bad", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeOrigin(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("normalizeOrigin(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ── intEnv ────────────────────────────────────────────────────────────────────

func TestIntEnv(t *testing.T) {
	const key = "TEST_INTENV_VAR"

	t.Run("returns default when unset", func(t *testing.T) {
		os.Unsetenv(key)
		if got := intEnv(key, 42); got != 42 {
			t.Errorf("got %d, want 42", got)
		}
	})

	t.Run("returns parsed value", func(t *testing.T) {
		os.Setenv(key, "99")
		defer os.Unsetenv(key)
		if got := intEnv(key, 42); got != 99 {
			t.Errorf("got %d, want 99", got)
		}
	})

	t.Run("returns default on non-numeric", func(t *testing.T) {
		os.Setenv(key, "abc")
		defer os.Unsetenv(key)
		if got := intEnv(key, 42); got != 42 {
			t.Errorf("got %d, want 42", got)
		}
	})

	t.Run("returns default on zero", func(t *testing.T) {
		os.Setenv(key, "0")
		defer os.Unsetenv(key)
		if got := intEnv(key, 42); got != 42 {
			t.Errorf("got %d, want 42 (zero treated as unset)", got)
		}
	})

	t.Run("returns default on negative", func(t *testing.T) {
		os.Setenv(key, "-5")
		defer os.Unsetenv(key)
		if got := intEnv(key, 42); got != 42 {
			t.Errorf("got %d, want 42 (negative treated as unset)", got)
		}
	})
}

// ── pct ───────────────────────────────────────────────────────────────────────

func TestPct(t *testing.T) {
	tests := []struct{ f float64; want string }{
		{0, "0%"},
		{1, "100%"},
		{0.5, "50%"},
		{0.333, "33%"},
		{0.999, "100%"},
	}
	for _, tt := range tests {
		if got := pct(tt.f); got != tt.want {
			t.Errorf("pct(%.3f) = %q, want %q", tt.f, got, tt.want)
		}
	}
}

// ── stripScheme ───────────────────────────────────────────────────────────────

func TestStripScheme(t *testing.T) {
	tests := []struct{ input, want string }{
		{"https://example.com", "example.com"},
		{"http://example.com", "example.com"},
		{"https://example.com/", "example.com"},
		{"example.com", "example.com"},
	}
	for _, tt := range tests {
		if got := stripScheme(tt.input); got != tt.want {
			t.Errorf("stripScheme(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ── buildReport ───────────────────────────────────────────────────────────────

func TestBuildReport_BasicOnly(t *testing.T) {
	basic := hScore{
		totalEntries:   10,
		meanDescWords:  8.5,
		thisPageRate:   0.1,
		blankDescs:     1,
		uniqueDescRate: 0.9,
		sectionCount:   3,
	}
	report := buildReport("https://example.com", 5*time.Second, 10, basic, hScore{}, false, nil, nil, nil)

	mustContain(t, report, "# Eval: example.com")
	mustContain(t, report, "Pages: 10")
	mustContain(t, report, "## Heuristic Scores")
	mustContain(t, report, "10%") // thisPageRate
	mustContain(t, report, "Enhanced mode not available")
	mustNotContain(t, report, "## Ground Truth")
	mustNotContain(t, report, "## LLM-as-Judge")
}

func TestBuildReport_WithEnhanced(t *testing.T) {
	basic := hScore{totalEntries: 10, thisPageRate: 0.2, sectionCount: 3}
	enhanced := hScore{totalEntries: 10, thisPageRate: 0.0, sectionCount: 3}

	report := buildReport("https://example.com", 3*time.Second, 10, basic, enhanced, true, nil, nil, nil)

	mustContain(t, report, "| Basic | Enhanced |")
	mustNotContain(t, report, "Enhanced mode not available")
}

func TestBuildReport_WithGroundTruth(t *testing.T) {
	basic := hScore{sectionCount: 2}
	gt := &doc{
		entries:  []evalEntry{{}, {}},
		sections: []string{"Docs", "Blog"},
	}
	comp := &gtResult{urlCoverage: 0.75, sectionAlignment: 0.5, falsePositives: 3}

	report := buildReport("https://example.com", time.Second, 5, basic, hScore{}, false, gt, comp, nil)

	mustContain(t, report, "## Ground Truth Comparison")
	mustContain(t, report, "75%") // urlCoverage
	mustContain(t, report, "50%") // sectionAlignment
	mustContain(t, report, "3")   // falsePositives
	mustNotContain(t, report, "## LLM-as-Judge")
}

func TestBuildReport_WithJudge(t *testing.T) {
	basic := hScore{}
	gt := &doc{}
	comp := &gtResult{}
	judge := &judgeReport{ourWins: 14, gtWins: 4, ties: 2, total: 20}

	report := buildReport("https://example.com", time.Second, 5, basic, hScore{}, false, gt, comp, judge)

	mustContain(t, report, "## LLM-as-Judge")
	mustContain(t, report, "Sampled 20")
	mustContain(t, report, "14") // ourWins
	mustContain(t, report, "4")  // gtWins
}

func mustContain(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("report missing %q", substr)
	}
}

func mustNotContain(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("report should not contain %q", substr)
	}
}
