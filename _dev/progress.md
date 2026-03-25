# Build Progress

## 1. `crawler/` package
- [x] `extractor.go` — HTML → title, meta description, body text, links
- [x] `robots.go` — Parse robots.txt, filter disallowed URLs
- [x] `sitemap.go` — Discover/parse sitemap.xml to seed the crawl queue
- [x] `crawler.go` — BFS loop with depth/page limits and crawl delay

## 2. `clients/llm/` package
- [x] `client.go` — Unified Anthropic/OpenAI client with prompt construction
- [x] `pool.go` — Semaphore-bounded goroutine pool for concurrent LLM calls

## 3. `services/generator/` package
- [x] `service.go` — Section inference, llms.txt formatting, crawl → describe → format pipeline

## 4. Wire it up
- [x] Crawler integrated into `/generate` handler with live SSE progress
- [x] LLM client instantiated and injected into generator service
- [x] Enhanced-mode pipeline live — real LLM descriptions replace meta text
- [x] Basic mode preserved and working; enhanced falls back gracefully if no LLM client

---

## Remaining / Nice-to-have

### Bugs
- BUG-002: `fetchRobots` uses `client.Get` instead of `http.NewRequestWithContext` (ignores context)

### Crawler improvements
- See `_dev/concurrent-crawling.md` — parallel fetching with worker pool, shared rate limiter, and UI speed control (Standard / Fast / Turbo)
- See `_dev/crawl-stats-summary.md` — structured stats in `done` SSE event + UI stat grid

### Observability
- See `_dev/metrics.md` — Prometheus metrics for request count, latency, LLM call rate/errors, pages crawled

### Prompt / output quality
- Seed prompt with page URL + title for richer context (fields already on `crawler.Page`)
- See `_dev/evaluation.md` — eval framework: heuristic scoring of basic vs enhanced, LLM-as-judge comparison against ground-truth llms.txt files from real sites
