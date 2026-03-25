# Concurrent Crawling

## Summary

Replace the current sequential BFS fetch loop with a concurrent worker pool bounded by a semaphore, reducing total crawl time significantly for sites with many pages.

## Motivation

Every page fetch is currently sequential — we wait for one HTTP response before sending the next request. Network I/O dominates the runtime, so concurrency should yield near-linear speedup up to the point where the target server becomes the bottleneck.

## Design

**Fan-out / fan-in coordinator.** Keep a single coordinator goroutine owning the queue and visited set (no mutex needed). It fans work out to a fixed-size worker pool via a channel and collects results back on a results channel. BFS level ordering is not guaranteed but doesn't affect output quality — sections are grouped by URL path segment, not crawl order.

**Rate limiting.** A simple `time.Sleep` per worker doesn't give the same per-domain throttle as the current sequential delay. Replace it with a `time.Ticker`-based rate limiter shared across all workers so the effective request rate stays at `1 / CRAWL_DELAY_MS` regardless of concurrency.

**Crawl-delay in robots.txt.** We currently ignore the `Crawl-delay` directive. With concurrency this becomes more important — parse it and use it as a floor for the rate limiter.

**Concurrency knob.** Add `Concurrency int` to `crawler.Config` (default 1, preserving current behavior). The server can expose this via a new `CRAWL_CONCURRENCY` env var.

**UI control.** Expose as a "Crawl speed" toggle in the UI — three levels:
- Standard (1 worker, ~500ms delay) — polite, safe for any site
- Fast (3 workers, ~200ms delay) — good for most sites, may cause 429s on rate-limited APIs
- Turbo (5 workers, ~100ms delay) — fastest, results may vary

Show a hint next to Fast/Turbo: "faster crawl, may miss pages on rate-limited sites". This gives users control without exposing raw numbers and sets expectations about tradeoffs.

## Considerations

- Start conservative — 3–5 workers gives most of the speedup without risking 429s
- `OnPage` callback must be safe to call from multiple goroutines (it currently just sends on an SSE writer, which is already serialized in the handler)
- The visited set uses `mapset` which is thread-safe, so no changes needed there
