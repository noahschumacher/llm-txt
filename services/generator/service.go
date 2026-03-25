package generator

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/noahschumacher/llm-txt/clients/llm"
	"github.com/noahschumacher/llm-txt/crawler"
)

// Service runs the crawl → describe → format pipeline.
type Service struct {
	log         *zap.Logger
	llmClient   llm.Describer // nil in basic mode
	concurrency int
}

// New returns a Service. llmClient may be nil for basic mode.
func New(log *zap.Logger, llmClient llm.Describer, concurrency int) *Service {
	return &Service{log: log, llmClient: llmClient, concurrency: concurrency}
}

// Result holds the generated llms.txt and a human-readable summary.
type Result struct {
	LLMsTxt string
	Summary string
}

// Options controls per-request generation behaviour.
type Options struct {
	Mode       string // "basic" | "enhanced"
	Origin     string
	OnCrawl    func(crawled int)   // progress: page crawled
	OnDescribe func(completed int) // progress: description generated
}

// Generate runs the full pipeline for the given pages.
func (s *Service) Generate(ctx context.Context, pages []crawler.Page, opts Options) Result {
	if opts.Mode == "enhanced" && s.llmClient != nil {
		pages = s.describe(ctx, pages, opts.Origin, opts.OnDescribe)
	}

	llmsTxt := formatLLMsTxt(opts.Origin, pages)
	return Result{
		LLMsTxt: llmsTxt,
		Summary: fmt.Sprintf("%d pages crawled · mode: %s", len(pages), opts.Mode),
	}
}

// describe replaces each page's Description with an LLM-generated one.
// The root page is skipped so its original meta description is preserved.
func (s *Service) describe(ctx context.Context, pages []crawler.Page, origin string, onDone func(int)) []crawler.Page {
	ctxs := make([]llm.PageContext, len(pages))
	for i, p := range pages {
		if p.URL != origin && p.URL != origin+"/" {
			ctxs[i] = llm.PageContext{URL: p.URL, Title: p.Title, Body: p.Body}
		}
	}

	descs := llm.DescribeAll(ctx, s.llmClient, ctxs, s.concurrency, onDone, s.log)

	// copy pages so we don't mutate the caller's slice
	out := make([]crawler.Page, len(pages))
	copy(out, pages)
	for i, d := range descs {
		if d != "" {
			out[i].Description = d
		}
	}
	return out
}

// formatLLMsTxt assembles a llms.txt from crawled pages.
func formatLLMsTxt(origin string, pages []crawler.Page) string {
	var rootDesc string
	for _, p := range pages {
		if p.URL == origin || p.URL == origin+"/" {
			rootDesc = p.Description
			break
		}
	}

	// order tracks section insertion order; sections holds pages per section.
	// maps don't preserve insertion order, so we maintain a separate slice.
	order := make([]string, 0, len(pages))
	sections := make(map[string][]crawler.Page, len(pages))

	for _, p := range pages {
		isRoot := p.URL == origin || p.URL == origin+"/"
		if isRoot {
			continue
		}
		if p.Description == rootDesc && rootDesc != "" {
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

func sectionName(u, origin string) string {
	path := strings.TrimPrefix(u, origin)
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "General"
	}
	seg := strings.SplitN(path, "/", 2)[0]
	seg = strings.SplitN(seg, "?", 2)[0] // strip query string if no leading slash
	if seg == "" {
		return "General"
	}
	return strings.ToUpper(seg[:1]) + seg[1:]
}

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
