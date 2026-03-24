# Crawl Stats Summary

## Summary

Return a structured stats payload at the end of each generation request and display it in the UI below the output — giving users a quick sense of what was crawled and how long it took.

## Motivation

The current summary line ("50 pages crawled · mode: basic") is minimal. Richer stats help users understand crawl quality (e.g. how many pages were skipped vs fetched) and set expectations for future runs.

## Proposed Stats

| Stat | Description |
|---|---|
| Pages fetched | Pages that returned 200 HTML and were added to output |
| Pages skipped | URLs that were dequeued but dropped (non-HTML, robots, skip patterns, errors) |
| Total time | Wall time from crawl start to llms.txt assembly complete |
| Sections found | Number of distinct URL path segments grouped into sections |
| Sitemap seeded | Whether the queue was seeded from sitemap.xml |
| Robots rules | Number of Disallow rules parsed from robots.txt |

## Design

**Backend.** `Crawler` accumulates a `Stats` struct during `Crawl` and returns it alongside `[]Page`. The handler includes it in the `done` SSE event as a new `stats` field.

```go
type Stats struct {
    PagesFetched  int
    PagesSkipped  int
    Duration      time.Duration
    SitemapSeeded bool
    RobotsRules   int
}
```

**SSE event.** Add `Stats` to the `done` event so it arrives alongside `llms_txt`.

**UI.** Replace the single summary line below the output textarea with a small stat grid — each stat as a label/value pair. Keep it compact so it doesn't distract from the output itself.
