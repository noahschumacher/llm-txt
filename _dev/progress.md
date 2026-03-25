# Build Progress

## 1. `crawler/` package
- [x] `extractor.go` — HTML → title, meta description, body text, links
- [x] `robots.go` — Parse robots.txt, filter disallowed URLs
- [x] `sitemap.go` — Discover/parse sitemap.xml to seed the crawl queue
- [x] `crawler.go` — BFS loop with depth/page limits and crawl delay

## 2. `clients/llm/` package
- [ ] `client.go` — Unified client for Anthropic/OpenAI with prompt construction
- [ ] `pool.go` — Semaphore-bounded goroutine pool for concurrent LLM calls

## 3. `server/services/generator/` package
- [ ] `sections.go` — Infer section names from URL path segments (`/docs/*` → Docs)
- [ ] `formatter.go` — Assemble llms.txt per spec
- [ ] `service.go` — Wire the full crawl → describe → format pipeline

## 4. Wire it up
- [x] Crawler integrated into `/generate` handler with live SSE progress
- [ ] LLM client instantiated and injected into generator service
- [ ] Mock output in `generate.go` replaced with real enhanced-mode pipeline

---

## Notes

- Sections and formatting logic currently live directly in `server/generate.go` — should be extracted into `server/services/generator/` when the LLM client is added (step 3)
- See `_dev/concurrent-crawling.md` and `_dev/crawl-stats-summary.md` for planned improvements to the crawler
