# Observability & Metrics

## Summary

Instrument the server with Prometheus metrics to track usage, performance, and error rates across generation requests.

## Motivation

Currently the only observability is structured logs. Metrics give a time-series view useful for dashboards, alerting, and understanding real-world usage patterns (e.g. which mode is most used, where latency is coming from, error rates by provider).

## Proposed Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `llmtxt_requests_total` | Counter | `mode`, `status` (success/error) | Total generation requests |
| `llmtxt_request_duration_seconds` | Histogram | `mode` | End-to-end request latency |
| `llmtxt_crawl_pages_total` | Histogram | — | Pages crawled per request |
| `llmtxt_crawl_duration_seconds` | Histogram | — | Time spent in crawl phase |
| `llmtxt_llm_calls_total` | Counter | `provider`, `status` (success/error) | LLM API calls |
| `llmtxt_llm_duration_seconds` | Histogram | `provider` | Per-call LLM latency |
| `llmtxt_llm_describe_duration_seconds` | Histogram | — | Total time in describe phase |

## Design

**Package.** Add `pkg/metrics/metrics.go` — registers all collectors once at startup, exposes a `Record*` helper per metric. Keeps Prometheus details out of business logic.

**Injection.** Pass a `*metrics.Metrics` into the server and generator service, same pattern as the logger.

**Endpoint.** Add `GET /metrics` via `promhttp.Handler()` — no timeout middleware (scrape should be fast but shouldn't time out the handler).

**Dependencies.** `github.com/prometheus/client_golang` — standard Go Prometheus client.

## Notes

- Histograms over summaries — histograms are aggregatable across instances; summaries are not
- Default buckets are fine for duration; crawl pages can use custom buckets (5, 10, 25, 50, 100, 200)
- In local env, `/metrics` output doubles as a quick sanity check after a run
