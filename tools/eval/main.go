package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/noahschumacher/llm-txt/clients/llm"
	"github.com/noahschumacher/llm-txt/crawler"
	"github.com/noahschumacher/llm-txt/services/generator"
)

func main() {
	urlFlag := flag.String("url", "", "site to evaluate (required)")
	gtFlag := flag.String("ground-truth", "", "URL of a published llms.txt to compare against")
	judgeFlag := flag.Bool("llm-judge", false, "enable LLM-as-judge scoring (requires --ground-truth)")
	outFlag := flag.String("out", "", "write report to file instead of stdout")
	envFlag := flag.String("env", "", "path to .env file")
	flag.Parse()

	if *urlFlag == "" {
		fmt.Fprintln(os.Stderr, "error: --url is required")
		flag.Usage()
		os.Exit(1)
	}
	if *judgeFlag && *gtFlag == "" {
		fmt.Fprintln(os.Stderr, "error: --llm-judge requires --ground-truth")
		os.Exit(1)
	}

	if *envFlag != "" {
		if err := godotenv.Load(*envFlag); err != nil {
			log.Fatalf("error loading env file: %v", err)
		}
	}

	llmProvider := os.Getenv("LLM_PROVIDER")
	llmAPIKey := os.Getenv("LLM_API_KEY")
	llmModel := os.Getenv("LLM_MODEL")
	llmConcurrency := intEnv("LLM_CONCURRENCY", 5)

	crawlCfg := crawler.NewConfig(
		intEnv("CRAWL_MAX_PAGES", 50),
		intEnv("CRAWL_MAX_DEPTH", 3),
		intEnv("CRAWL_DELAY_MS", 200),
		intEnv("CRAWL_CONCURRENCY", 3),
	)

	nop := zap.NewNop()

	var llmClient llm.Describer
	if llmProvider != "" && llmAPIKey != "" {
		var err error
		llmClient, err = llm.New(llmProvider, llmAPIKey, llmModel, nop)
		if err != nil {
			log.Fatalf("error creating llm client: %v", err)
		}
	}

	ctx := context.Background()

	siteURL, err := normalizeOrigin(*urlFlag)
	if err != nil {
		log.Fatalf("invalid url: %v", err)
	}

	// ── Crawl ────────────────────────────────────────────────────────────────

	fmt.Fprintf(os.Stderr, "crawling %s...\n", siteURL)
	c := crawler.New(crawlCfg, nop)
	crawlStart := time.Now()
	pages, err := c.Crawl(ctx, siteURL)
	crawlDuration := time.Since(crawlStart)
	if err != nil && len(pages) == 0 {
		log.Fatalf("crawl failed: %v", err)
	}
	fmt.Fprintf(os.Stderr, "crawled %d pages in %s\n", len(pages), crawlDuration.Round(time.Millisecond))

	// ── Generate ─────────────────────────────────────────────────────────────

	basicGen := generator.New(nop, nil, 0)
	basicResult := basicGen.Generate(ctx, pages, generator.Options{
		Mode:   "basic",
		Origin: siteURL,
	})
	basicDoc := parseLLMsTxt(basicResult.LLMsTxt)
	basicScore := scoreHeuristics(basicDoc)

	var enhancedDoc doc
	var enhancedScore hScore
	hasEnhanced := llmClient != nil
	if hasEnhanced {
		fmt.Fprintln(os.Stderr, "generating enhanced descriptions...")
		enhGen := generator.New(nop, llmClient, llmConcurrency)
		enhResult := enhGen.Generate(ctx, pages, generator.Options{
			Mode:   "enhanced",
			Origin: siteURL,
		})
		enhancedDoc = parseLLMsTxt(enhResult.LLMsTxt)
		enhancedScore = scoreHeuristics(enhancedDoc)
	}

	// ── Ground truth ─────────────────────────────────────────────────────────

	var gt *doc
	var gtComp *gtResult
	if *gtFlag != "" {
		fmt.Fprintf(os.Stderr, "fetching ground truth from %s...\n", *gtFlag)
		gtDoc, err := fetchGroundTruth(ctx, *gtFlag)
		if err != nil {
			log.Fatalf("failed to fetch ground truth: %v", err)
		}
		gt = &gtDoc

		pageBodies := make(map[string]string, len(pages))
		for _, p := range pages {
			pageBodies[p.URL] = p.Body
		}

		// compare against enhanced when available, basic otherwise
		compareDoc := basicDoc
		if hasEnhanced {
			compareDoc = enhancedDoc
		}
		comp := compareGroundTruth(compareDoc, *gt, pageBodies)
		gtComp = &comp
	}

	// ── LLM-as-judge ─────────────────────────────────────────────────────────

	var judge *judgeReport
	if *judgeFlag && gtComp != nil {
		completer, ok := llmClient.(llm.Completer)
		if !ok {
			fmt.Fprintln(os.Stderr, "warning: llm client does not implement Completer, skipping judge")
		} else {
			n := len(gtComp.overlap)
			fmt.Fprintf(os.Stderr, "running llm-as-judge on %d overlapping pages (up to 20 sampled)...\n", n)
			r := runJudge(ctx, completer, gtComp.overlap, 20)
			judge = &r
		}
	}

	// ── Report ────────────────────────────────────────────────────────────────

	report := buildReport(siteURL, crawlDuration, len(pages), basicScore, enhancedScore, hasEnhanced, gt, gtComp, judge)

	if *outFlag != "" {
		if err := os.WriteFile(*outFlag, []byte(report), 0o644); err != nil {
			log.Fatalf("failed to write report: %v", err)
		}
		fmt.Fprintf(os.Stderr, "report written to %s\n", *outFlag)
	} else {
		fmt.Print(report)
	}
}

func buildReport(
	siteURL string,
	crawlDuration time.Duration,
	pageCount int,
	basic, enhanced hScore,
	hasEnhanced bool,
	gt *doc,
	gtComp *gtResult,
	judge *judgeReport,
) string {
	var b strings.Builder
	host := stripScheme(siteURL)

	fmt.Fprintf(&b, "# Eval: %s\n\n", host)
	fmt.Fprintf(&b, "Generated: %s\n\n", time.Now().Format(time.RFC3339))

	fmt.Fprintf(&b, "## Crawl\n\n")
	fmt.Fprintf(&b, "- Pages: %d\n", pageCount)
	fmt.Fprintf(&b, "- Duration: %s\n", crawlDuration.Round(time.Millisecond))
	fmt.Fprintf(&b, "- Sections: %d\n\n", basic.sectionCount)

	fmt.Fprintf(&b, "## Heuristic Scores\n\n")
	if hasEnhanced {
		fmt.Fprintf(&b, "| Dimension | Basic | Enhanced |\n")
		fmt.Fprintf(&b, "|---|---|---|\n")
		fmt.Fprintf(&b, "| Total entries | %d | %d |\n", basic.totalEntries, enhanced.totalEntries)
		fmt.Fprintf(&b, "| Mean description words | %.1f | %.1f |\n", basic.meanDescWords, enhanced.meanDescWords)
		fmt.Fprintf(&b, "| \"This page...\" prefix rate | %s | %s |\n", pct(basic.thisPageRate), pct(enhanced.thisPageRate))
		fmt.Fprintf(&b, "| Blank descriptions | %d | %d |\n", basic.blankDescs, enhanced.blankDescs)
		fmt.Fprintf(&b, "| Unique descriptions | %s | %s |\n", pct(basic.uniqueDescRate), pct(enhanced.uniqueDescRate))
		fmt.Fprintf(&b, "| Section count | %d | %d |\n\n", basic.sectionCount, enhanced.sectionCount)
	} else {
		fmt.Fprintf(&b, "| Dimension | Basic |\n")
		fmt.Fprintf(&b, "|---|---|\n")
		fmt.Fprintf(&b, "| Total entries | %d |\n", basic.totalEntries)
		fmt.Fprintf(&b, "| Mean description words | %.1f |\n", basic.meanDescWords)
		fmt.Fprintf(&b, "| \"This page...\" prefix rate | %s |\n", pct(basic.thisPageRate))
		fmt.Fprintf(&b, "| Blank descriptions | %d |\n", basic.blankDescs)
		fmt.Fprintf(&b, "| Unique descriptions | %s |\n", pct(basic.uniqueDescRate))
		fmt.Fprintf(&b, "| Section count | %d |\n\n", basic.sectionCount)
		b.WriteString("_Enhanced mode not available — set LLM_PROVIDER and LLM_API_KEY to enable._\n\n")
	}

	if gt != nil && gtComp != nil {
		fmt.Fprintf(&b, "## Ground Truth Comparison\n\n")
		fmt.Fprintf(&b, "Ground truth: %d entries · %d sections\n\n", len(gt.entries), len(gt.sections))
		fmt.Fprintf(&b, "| Dimension | Value |\n")
		fmt.Fprintf(&b, "|---|---|\n")
		fmt.Fprintf(&b, "| URL coverage | %s |\n", pct(gtComp.urlCoverage))
		fmt.Fprintf(&b, "| Section alignment | %s |\n", pct(gtComp.sectionAlignment))
		fmt.Fprintf(&b, "| False positives | %d |\n\n", gtComp.falsePositives)
	}

	if judge != nil {
		fmt.Fprintf(&b, "## LLM-as-Judge\n\n")
		fmt.Fprintf(&b, "Sampled %d overlapping pages.\n\n", judge.total)
		fmt.Fprintf(&b, "| Verdict | Count |\n")
		fmt.Fprintf(&b, "|---|---|\n")
		fmt.Fprintf(&b, "| Ours preferred | %d |\n", judge.ourWins)
		fmt.Fprintf(&b, "| Ground truth preferred | %d |\n", judge.gtWins)
		fmt.Fprintf(&b, "| Tie | %d |\n\n", judge.ties)
	}

	return b.String()
}

func normalizeOrigin(raw string) (string, error) {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host, nil
}

func intEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func pct(f float64) string {
	return fmt.Sprintf("%.0f%%", f*100)
}

func stripScheme(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.TrimSuffix(u, "/")
}
