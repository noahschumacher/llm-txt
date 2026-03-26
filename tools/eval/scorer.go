package main

import (
	"regexp"
	"strings"
)

// doc is a parsed llms.txt document.
type doc struct {
	host     string
	rootDesc string
	sections []string
	entries  []evalEntry
}

type evalEntry struct {
	title   string
	url     string
	desc    string
	section string
}

var entryRE = regexp.MustCompile(`^- \[([^\]]*)\]\(([^)]+)\)(?:: (.+))?$`)

// parseLLMsTxt parses an llms.txt string into a doc.
func parseLLMsTxt(text string) doc {
	var d doc
	var currentSection string
	seenSections := make(map[string]struct{})

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "# "):
			d.host = strings.TrimPrefix(line, "# ")
		case strings.HasPrefix(line, "> "):
			d.rootDesc = strings.TrimPrefix(line, "> ")
		case strings.HasPrefix(line, "## "):
			currentSection = strings.TrimPrefix(line, "## ")
			if _, seen := seenSections[currentSection]; !seen {
				seenSections[currentSection] = struct{}{}
				d.sections = append(d.sections, currentSection)
			}
		default:
			m := entryRE.FindStringSubmatch(line)
			if m != nil {
				d.entries = append(d.entries, evalEntry{
					title:   m[1],
					url:     m[2],
					desc:    m[3],
					section: currentSection,
				})
			}
		}
	}
	return d
}

type hScore struct {
	totalEntries   int
	meanDescWords  float64
	thisPageRate   float64 // fraction of entries starting with "this page"
	blankDescs     int
	uniqueDescRate float64 // fraction of non-blank descriptions that are distinct
	sectionCount   int
}

// scoreHeuristics computes offline quality metrics for a parsed llms.txt.
func scoreHeuristics(d doc) hScore {
	s := hScore{
		totalEntries: len(d.entries),
		sectionCount: len(d.sections),
	}
	if len(d.entries) == 0 {
		return s
	}

	var totalWords, thisPageCount int
	descSet := make(map[string]struct{}, len(d.entries))

	for _, e := range d.entries {
		if e.desc == "" {
			s.blankDescs++
			continue
		}
		totalWords += len(strings.Fields(e.desc))
		if strings.HasPrefix(strings.ToLower(e.desc), "this page") {
			thisPageCount++
		}
		descSet[strings.ToLower(strings.TrimSpace(e.desc))] = struct{}{}
	}

	nonBlank := len(d.entries) - s.blankDescs
	if nonBlank > 0 {
		s.meanDescWords = float64(totalWords) / float64(nonBlank)
		s.uniqueDescRate = float64(len(descSet)) / float64(nonBlank)
	}
	s.thisPageRate = float64(thisPageCount) / float64(len(d.entries))

	return s
}
