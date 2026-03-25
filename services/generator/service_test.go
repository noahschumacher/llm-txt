package generator

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/noahschumacher/llm-txt/clients/llm"
	"github.com/noahschumacher/llm-txt/crawler"
)

// mockDescriber returns a fixed description for every page.
type mockDescriber struct {
	desc  string
	calls int
}

func (m *mockDescriber) Describe(_ context.Context, _ llm.PageContext) (string, error) {
	m.calls++
	return m.desc, nil
}

var testPages = []crawler.Page{
	{URL: "https://example.com", Title: "Example", Description: "The root page.", Body: "Root body."},
	{URL: "https://example.com/docs/intro", Title: "Intro", Description: "Meta desc.", Body: "Intro body."},
	{URL: "https://example.com/blog/post", Title: "Post", Description: "", Body: "Post body."},
}

func TestGenerate_BasicMode(t *testing.T) {
	mock := &mockDescriber{desc: "LLM description."}
	svc := New(zap.NewNop(), mock, 1)

	result := svc.Generate(context.Background(), testPages, Options{
		Mode:   "basic",
		Origin: "https://example.com",
	})

	if mock.calls != 0 {
		t.Errorf("basic mode should not call LLM, got %d calls", mock.calls)
	}
	if !strings.Contains(result.LLMsTxt, "# example.com") {
		t.Errorf("expected site header, got:\n%s", result.LLMsTxt)
	}
	if !strings.Contains(result.LLMsTxt, "Meta desc.") {
		t.Errorf("expected meta description from page, got:\n%s", result.LLMsTxt)
	}
	if strings.Contains(result.LLMsTxt, "LLM description.") {
		t.Errorf("basic mode should not contain LLM-generated text, got:\n%s", result.LLMsTxt)
	}
}

func TestGenerate_EnhancedMode(t *testing.T) {
	mock := &mockDescriber{desc: "LLM description."}
	svc := New(zap.NewNop(), mock, 2)

	result := svc.Generate(context.Background(), testPages, Options{
		Mode:   "enhanced",
		Origin: "https://example.com",
	})

	// root page is skipped — LLM called for non-root pages only
	wantCalls := len(testPages) - 1
	if mock.calls != wantCalls {
		t.Errorf("expected %d LLM calls, got %d", wantCalls, mock.calls)
	}
	if !strings.Contains(result.LLMsTxt, "LLM description.") {
		t.Errorf("expected LLM-generated description in output, got:\n%s", result.LLMsTxt)
	}
}

func TestGenerate_EnhancedMode_NoClient(t *testing.T) {
	svc := New(zap.NewNop(), nil, 1)

	result := svc.Generate(context.Background(), testPages, Options{
		Mode:   "enhanced",
		Origin: "https://example.com",
	})

	// should fall back to basic formatting
	if !strings.Contains(result.LLMsTxt, "# example.com") {
		t.Errorf("expected site header in fallback output, got:\n%s", result.LLMsTxt)
	}
}

func TestGenerate_OnDescribeCallback(t *testing.T) {
	mock := &mockDescriber{desc: "A description."}
	svc := New(zap.NewNop(), mock, 2)

	var progressUpdates []int
	svc.Generate(context.Background(), testPages, Options{
		Mode:   "enhanced",
		Origin: "https://example.com",
		OnDescribe: func(completed int) {
			progressUpdates = append(progressUpdates, completed)
		},
	})

	wantUpdates := len(testPages) // onDone fires for every slot including skipped root
	if len(progressUpdates) != wantUpdates {
		t.Errorf("expected %d progress updates, got %d", wantUpdates, len(progressUpdates))
	}
}

func TestSectionName(t *testing.T) {
	tests := []struct {
		name   string
		u      string
		origin string
		want   string
	}{
		{
			name:   "docs segment",
			u:      "https://example.com/docs/getting-started",
			origin: "https://example.com",
			want:   "Docs",
		},
		{
			name:   "capitalises first letter",
			u:      "https://example.com/blog/my-post",
			origin: "https://example.com",
			want:   "Blog",
		},
		{
			name:   "root path → General",
			u:      "https://example.com/",
			origin: "https://example.com",
			want:   "General",
		},
		{
			name:   "no path → General",
			u:      "https://example.com",
			origin: "https://example.com",
			want:   "General",
		},
		{
			name:   "query string on segment",
			u:      "https://example.com/search?q=foo",
			origin: "https://example.com",
			want:   "Search",
		},
		{
			name:   "single segment no trailing slash",
			u:      "https://example.com/about",
			origin: "https://example.com",
			want:   "About",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sectionName(tt.u, tt.origin)
			if got != tt.want {
				t.Errorf("sectionName(%q, %q) = %q, want %q", tt.u, tt.origin, got, tt.want)
			}
		})
	}
}

func TestFirstSentence(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "ends at period",
			text: "This is a sentence. This is another.",
			want: "This is a sentence.",
		},
		{
			name: "ends at exclamation",
			text: "Hello! World.",
			want: "Hello!",
		},
		{
			name: "ends at question mark",
			text: "What is this? Something.",
			want: "What is this?",
		},
		{
			name: "no sentence ending — short text returned as-is",
			text: "No punctuation here",
			want: "No punctuation here",
		},
		{
			name: "no sentence ending — truncates at 160",
			text: "This is a very long line of text that goes on and on without any punctuation whatsoever and will eventually exceed the one hundred and sixty character truncation limit enforced by this function",
			want: "This is a very long line of text that goes on and on without any punctuation whatsoever and will eventually exceed the one hundred and sixty character truncatio...",
		},
		{
			name: "empty string",
			text: "",
			want: "",
		},
		{
			name: "leading whitespace trimmed",
			text: "   Hello world.",
			want: "Hello world.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstSentence(tt.text)
			if got != tt.want {
				t.Errorf("firstSentence(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}
