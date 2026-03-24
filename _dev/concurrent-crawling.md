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

## Considerations

- Start conservative — 3–5 workers gives most of the speedup without risking 429s
- `OnPage` callback must be safe to call from multiple goroutines (it currently just sends on an SSE writer, which is already serialized in the handler)
- The visited set uses `mapset` which is thread-safe, so no changes needed there
