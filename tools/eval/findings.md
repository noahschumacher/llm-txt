# Eval Findings

## Sites Tested

| Site | Pages Crawled | GT Entries | URL Coverage | Notes |
|---|---|---|---|---|
| go.dev | 50 | — | — | No llms.txt published |
| stripe.com | 50 | 268 | 3% | Scale gap — GT is enormous |
| hono.dev | 27 | 86 | 29% | No meta descriptions — basic mode unusable |

---

## Key Findings

### Basic vs Enhanced

**Enhanced consistently tightens descriptions.**
- Mean word count drops from ~22–26 words (basic) down to ~11–12 (enhanced), matching our "under 15 words" prompt target.
- "This page..." prefix rate is 0% on both modes — the prompt is working.
- Enhanced hits 100% unique descriptions; basic is 94–100% depending on site.

**Basic mode breaks on doc-heavy sites.**
- hono.dev: 0 entries in basic mode because the site has no `<meta name="description">` tags. Every description was blank and filtered. Enhanced rescued all 26 entries by generating from body text.
- This is a real signal: for developer documentation sites, basic mode may produce an empty or near-empty output. Enhanced is not optional for these sites.

### URL Coverage vs Ground Truth

**Coverage numbers are primarily a scale story, not a quality story.**
- stripe.com: 3% coverage — we crawled 50 of their 268 indexed pages. The pages we found were legitimate; we just hit the crawl cap.
- hono.dev: 29% coverage — crawled 27 of 86 pages. More representative.
- Coverage only becomes meaningful when `CRAWL_MAX_PAGES` is set high enough to approach the ground truth size.

**False positives are low when crawl quality is high.**
- hono.dev: 1 false positive out of 26. We included one page they chose not to.
- stripe.com: 41 false positives — mostly marketing and blog pages Stripe intentionally excluded from their index.

### Section Alignment

**Our section inference (first URL path segment) diverges from hand-curated sections.**
- hono.dev GT sections: "Docs", "Examples", "Optional" → 33% alignment with our path-derived sections.
- stripe.com GT sections: product/feature names ("Payments", "Connect", etc.) vs our path segments.
- Their sections reflect product intent; ours reflect URL structure. These will rarely match on product marketing sites.
- For pure doc sites (uniform `/docs/*` paths), alignment is likely better.

### Crawl Behaviour

**JS-rendered sites are invisible to us.**
- elysiajs.com: 1 page crawled despite having 94-entry llms.txt. Site requires JS to render navigation links.
- This is a fundamental crawler limitation — we only follow `<a href>` links in raw HTML.

**Concurrent crawling works well.**
- go.dev: 50 pages in ~4–6s with Turbo (5 workers). Sequential would be ~25–50s.
- hono.dev: 27 pages in 1.9s.

---

## Open Questions

- Should the eval report flag when basic mode produces 0 or near-0 entries? That's a useful warning.
- URL coverage metric needs a note in the report when crawled pages < GT entries (scale gap caveat).
- Section alignment may be more useful as "any of our sections appear in GT" rather than strict fraction.
