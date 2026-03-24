# Bug Tracker

## Open

### BUG-002: fetchRobots ignores context
**Where:** `crawler/robots.go` → `fetchRobots`
**What:** Uses `client.Get` directly instead of `http.NewRequestWithContext`, so a cancelled context is not respected.
**Expected:** Use `http.NewRequestWithContext(ctx, ...)` like the rest of the crawler.

---

## Closed

### BUG-005: /issue/ paths crawled (GitHub redirect noise)
**Fixed in:** `crawler/crawler.go` → `skipPatterns`
**Fix:** Added `/issue/` and `/issues/` to skip patterns so issue tracker URLs are filtered before enqueue.

### BUG-004: .html and non-.html URLs treated as separate pages
**Fixed in:** `crawler/url.go` → `normalizeURL`
**Fix:** Strip `.html` suffix from the path before deduplication so `getting-started` and `getting-started.html` normalize to the same key.

### BUG-003: Nav/header/footer text pollutes body extraction
**Fixed in:** `crawler/extractor.go`
**Fix:** Added `nav`, `header`, `footer`, `aside` to the skip list so structural chrome is excluded from body text extraction.

### BUG-001: Section index pages show generic site description
**Fixed in:** `server/generate.go` → `formatLLMsTxt`
**Fix:** Extract root page description first. Skip any non-root page whose description matches it — these inherited the global meta tag and have no unique content.
