package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/noahschumacher/llm-txt/clients/llm"
)

type gtResult struct {
	urlCoverage      float64 // fraction of GT URLs present in our output
	sectionAlignment float64 // fraction of GT sections matched by ours
	falsePositives   int     // our URLs not in GT
	overlap          []overlapEntry
}

// overlapEntry holds a page present in both our output and the ground truth,
// used as input for LLM-as-judge scoring.
type overlapEntry struct {
	url     string
	body    string // from our crawl
	ourDesc string
	gtDesc  string
}

// fetchGroundTruth fetches and parses a published llms.txt.
func fetchGroundTruth(ctx context.Context, gtURL string) (doc, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gtURL, nil)
	if err != nil {
		return doc{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return doc{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return doc{}, fmt.Errorf("ground truth fetch returned %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return doc{}, err
	}
	return parseLLMsTxt(string(b)), nil
}

// compareGroundTruth scores our output against a ground truth doc.
// pageBodies maps URL → crawled body text, used to build overlap entries for judging.
func compareGroundTruth(ours, gt doc, pageBodies map[string]string) gtResult {
	ourURLs := make(map[string]string, len(ours.entries))
	for _, e := range ours.entries {
		ourURLs[e.url] = e.desc
	}

	gtURLs := make(map[string]struct{}, len(gt.entries))
	for _, e := range gt.entries {
		gtURLs[e.url] = struct{}{}
	}

	var covered int
	var overlap []overlapEntry
	for _, e := range gt.entries {
		ourDesc, inOurs := ourURLs[e.url]
		if !inOurs {
			continue
		}
		covered++
		if body := pageBodies[e.url]; body != "" && e.desc != "" && ourDesc != "" {
			overlap = append(overlap, overlapEntry{
				url:     e.url,
				body:    body,
				ourDesc: ourDesc,
				gtDesc:  e.desc,
			})
		}
	}

	var urlCoverage float64
	if len(gt.entries) > 0 {
		urlCoverage = float64(covered) / float64(len(gt.entries))
	}

	gtSections := make(map[string]struct{}, len(gt.sections))
	for _, s := range gt.sections {
		gtSections[strings.ToLower(s)] = struct{}{}
	}
	var sectionMatches int
	for _, s := range ours.sections {
		if _, ok := gtSections[strings.ToLower(s)]; ok {
			sectionMatches++
		}
	}
	var sectionAlignment float64
	if len(gt.sections) > 0 {
		sectionAlignment = float64(sectionMatches) / float64(len(gt.sections))
	}

	var falsePositives int
	for _, e := range ours.entries {
		if _, inGT := gtURLs[e.url]; !inGT {
			falsePositives++
		}
	}

	return gtResult{
		urlCoverage:      urlCoverage,
		sectionAlignment: sectionAlignment,
		falsePositives:   falsePositives,
		overlap:          overlap,
	}
}

const judgePrompt = `You are evaluating descriptions for an llms.txt index file.

Which description is more useful for an LLM navigating this site?

A: %s
B: %s

Page content (excerpt):
%s

Reply with exactly "A", "B", or "Tie" on the first line, followed by one sentence explaining why.`

type judgeReport struct {
	ourWins int
	gtWins  int
	ties    int
	total   int
}

// runJudge samples up to maxSamples overlapping pages and asks the LLM to
// pick our description vs the ground truth description.
func runJudge(ctx context.Context, c llm.Completer, overlap []overlapEntry, maxSamples int) judgeReport {
	samples := overlap
	if len(samples) > maxSamples {
		samples = samples[:maxSamples]
	}

	var r judgeReport
	r.total = len(samples)

	for _, o := range samples {
		body := o.body
		if len(body) > 2000 {
			body = body[:2000]
		}
		resp, err := c.Complete(ctx, fmt.Sprintf(judgePrompt, o.ourDesc, o.gtDesc, body))
		if err != nil {
			r.ties++ // count errors as ties rather than skipping
			continue
		}
		first := strings.ToUpper(strings.TrimSpace(strings.SplitN(resp, "\n", 2)[0]))
		switch first {
		case "A":
			r.ourWins++
		case "B":
			r.gtWins++
		default:
			r.ties++
		}
	}
	return r
}
