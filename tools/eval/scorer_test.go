package main

import (
	"testing"
)

func TestParseLLMsTxt(t *testing.T) {
	t.Run("full document", func(t *testing.T) {
		input := `# example.com

> Root description.

## Docs

- [Intro](https://example.com/docs/intro): Explains the intro.
- [Guide](https://example.com/docs/guide): Explains the guide.

## Blog

- [Post](https://example.com/blog/post): A blog post.
- [No Desc](https://example.com/blog/nodesc)
`
		d := parseLLMsTxt(input)

		if d.host != "example.com" {
			t.Errorf("host = %q, want %q", d.host, "example.com")
		}
		if d.rootDesc != "Root description." {
			t.Errorf("rootDesc = %q, want %q", d.rootDesc, "Root description.")
		}
		if len(d.sections) != 2 {
			t.Errorf("sections = %v, want 2", d.sections)
		}
		if len(d.entries) != 4 {
			t.Errorf("entries = %d, want 4", len(d.entries))
		}

		// entry with description
		e := d.entries[0]
		if e.title != "Intro" || e.url != "https://example.com/docs/intro" || e.desc != "Explains the intro." {
			t.Errorf("entry[0] = %+v, unexpected", e)
		}
		if e.section != "Docs" {
			t.Errorf("entry[0].section = %q, want Docs", e.section)
		}

		// entry without description
		e = d.entries[3]
		if e.desc != "" {
			t.Errorf("entry[3].desc = %q, want empty", e.desc)
		}
	})

	t.Run("no root description", func(t *testing.T) {
		input := "# example.com\n\n## Docs\n\n- [Page](https://example.com/page): Desc.\n"
		d := parseLLMsTxt(input)
		if d.rootDesc != "" {
			t.Errorf("rootDesc = %q, want empty", d.rootDesc)
		}
		if len(d.entries) != 1 {
			t.Errorf("entries = %d, want 1", len(d.entries))
		}
	})

	t.Run("duplicate sections deduplicated", func(t *testing.T) {
		input := "# example.com\n\n## Docs\n\n- [A](https://example.com/a): A.\n\n## Docs\n\n- [B](https://example.com/b): B.\n"
		d := parseLLMsTxt(input)
		if len(d.sections) != 1 {
			t.Errorf("sections = %v, want 1 (deduped)", d.sections)
		}
		// both entries should still be parsed
		if len(d.entries) != 2 {
			t.Errorf("entries = %d, want 2", len(d.entries))
		}
	})

	t.Run("empty string", func(t *testing.T) {
		d := parseLLMsTxt("")
		if d.host != "" || len(d.entries) != 0 || len(d.sections) != 0 {
			t.Errorf("expected empty doc, got %+v", d)
		}
	})
}

func TestScoreHeuristics(t *testing.T) {
	t.Run("empty doc", func(t *testing.T) {
		s := scoreHeuristics(doc{})
		if s.totalEntries != 0 || s.blankDescs != 0 || s.meanDescWords != 0 {
			t.Errorf("empty doc should produce zero score, got %+v", s)
		}
	})

	t.Run("all blank descriptions", func(t *testing.T) {
		d := doc{
			entries: []evalEntry{
				{url: "https://example.com/a"},
				{url: "https://example.com/b"},
			},
		}
		s := scoreHeuristics(d)
		if s.blankDescs != 2 {
			t.Errorf("blankDescs = %d, want 2", s.blankDescs)
		}
		if s.meanDescWords != 0 {
			t.Errorf("meanDescWords = %.1f, want 0", s.meanDescWords)
		}
	})

	t.Run("this page prefix rate", func(t *testing.T) {
		d := doc{
			entries: []evalEntry{
				{url: "https://example.com/a", desc: "This page covers foo."},
				{url: "https://example.com/b", desc: "This page explains bar."},
				{url: "https://example.com/c", desc: "Explains baz."},
				{url: "https://example.com/d", desc: "Covers qux."},
			},
		}
		s := scoreHeuristics(d)
		if s.thisPageRate != 0.5 {
			t.Errorf("thisPageRate = %.2f, want 0.50", s.thisPageRate)
		}
	})

	t.Run("unique description rate", func(t *testing.T) {
		d := doc{
			entries: []evalEntry{
				{url: "https://example.com/a", desc: "Unique A."},
				{url: "https://example.com/b", desc: "Unique B."},
				{url: "https://example.com/c", desc: "Unique A."}, // duplicate
			},
		}
		s := scoreHeuristics(d)
		// 2 distinct descriptions out of 3 non-blank
		want := 2.0 / 3.0
		if s.uniqueDescRate < want-0.01 || s.uniqueDescRate > want+0.01 {
			t.Errorf("uniqueDescRate = %.4f, want %.4f", s.uniqueDescRate, want)
		}
	})

	t.Run("mean description words", func(t *testing.T) {
		d := doc{
			entries: []evalEntry{
				{url: "https://example.com/a", desc: "One two three"},    // 3 words
				{url: "https://example.com/b", desc: "One two three four"}, // 4 words
				{url: "https://example.com/c"},                             // blank — excluded
			},
		}
		s := scoreHeuristics(d)
		// mean of 3 and 4 = 3.5
		if s.meanDescWords < 3.4 || s.meanDescWords > 3.6 {
			t.Errorf("meanDescWords = %.2f, want 3.5", s.meanDescWords)
		}
		if s.blankDescs != 1 {
			t.Errorf("blankDescs = %d, want 1", s.blankDescs)
		}
	})

	t.Run("section count", func(t *testing.T) {
		d := doc{sections: []string{"Docs", "Blog", "About"}}
		s := scoreHeuristics(d)
		if s.sectionCount != 3 {
			t.Errorf("sectionCount = %d, want 3", s.sectionCount)
		}
	})
}
