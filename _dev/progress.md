# Build Progress

## 1. `crawler/` package
- [x] `extractor.go` — HTML → title, meta description, body text, links
- [x] `robots.go` — Parse robots.txt, filter disallowed URLs
- [x] `sitemap.go` — Discover/parse sitemap.xml to seed the crawl queue
- [x] `crawler.go` — Concurrent ticker-paced BFS dispatcher with depth/page limits; off-domain redirect filtering

## 2. `clients/llm/` package
- [x] `client.go` — Unified Anthropic/OpenAI client; `PageContext{URL, Title, Body}` passed to LLM for richer prompt context; `Completer` interface for free-form LLM calls
- [x] `pool.go` — Semaphore-bounded goroutine pool for concurrent LLM calls

## 3. `services/generator/` package
- [x] `service.go` — Section inference, llms.txt formatting, crawl → describe → format pipeline

## 4. Wire it up
- [x] Crawler integrated into `/generate` handler with live SSE progress
- [x] LLM client instantiated and injected into generator service
- [x] Enhanced-mode pipeline live — real LLM descriptions (seeded with URL + title) replace meta text
- [x] Basic mode preserved and working; enhanced falls back gracefully if no LLM client
- [x] Concurrent crawling — `Concurrency` field on `crawler.Config`; ticker paces requests across workers
- [x] Crawl speed exposed in UI as Standard / Fast / Turbo radio buttons; sent as `concurrency` in request body
- [x] Max pages and max depth configurable from UI

## 5. `tools/eval/` — Evaluation CLI
- [x] `scorer.go` — `parseLLMsTxt`, `scoreHeuristics` — offline heuristic scoring
- [x] `groundtruth.go` — `fetchGroundTruth`, `compareGroundTruth`, `runJudge` — ground truth + LLM-as-judge
- [x] `main.go` — orchestration, `--url`, `--ground-truth`, `--llm-judge`, `--out` flags, markdown report
- [x] Full test suite — 34 tests passing across scorer, groundtruth, and main helpers
- [x] `make eval ARGS="..."` target
- [x] Ran against go.dev, stripe.com, hono.dev — see `tools/eval/findings.md`

---

## 6. Security & deployment
- [x] Password gate — `POST /password/check` endpoint; `X-Password` header required on `/generate`; `APP_PASSWORD` env var (default `"profound"`)
- [x] Frontend password screen — shown on load, unlocks app on success, bounces back on 401
- [x] Canonical origin resolution — `resolveOrigin` follows redirects at crawl start so `example.com → www.example.com` sites work correctly
- [x] Dockerfile added for containerized deployment

---

## Out of Scope (documented, not built)

These were designed and scoped but intentionally cut to keep the submission focused. Design docs are in `_dev/`.

### Known bug
- **BUG-002:** `fetchRobots` uses `client.Get` instead of `http.NewRequestWithContext` — robots.txt fetch ignores context cancellation. Low risk in practice (fast request, no long-running consequence).

### Observability — `_dev/metrics.md`
Prometheus `/metrics` endpoint: request count, latency histograms, LLM call rate/errors, pages crawled per request. Design is complete; skipped in favor of shipping core features.

### Crawl stats summary — `_dev/crawl-stats-summary.md`
Structured `stats` field in the `done` SSE event (pages fetched, pages skipped, duration, sitemap-seeded flag) and a small stat grid in the UI. Straightforward to add.

### Eval improvements (from findings)
- Warn in report when basic mode produces 0 entries (site has no meta descriptions)
- Add scale-gap caveat to URL coverage when crawled pages < GT entry count
- "Any section overlap" metric alongside strict alignment fraction
- Surface JS-rendering limitation in the UI
