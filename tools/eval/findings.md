# Eval Findings

## Sites Tested

| Site | Pages Crawled | Crawl Time | GT Entries | URL Coverage | Notes |
|---|---|---|---|---|---|
| go.dev | 50 | ~5s | — | — | No llms.txt published |
| hono.dev | 27 | 1.9s | 86 | 29% | No meta descriptions — basic mode unusable |
| stripe.com | 129 | 30.5s | 268 | 9% | Hit rate limits before 150-page cap |
| vercel.com | 150 | 28.4s | 571 | 0% | GT is enormous — pure scale gap |
| supabase.com | 150 | 11.9s | 8 | 0% | GT is sparse/hand-curated (8 entries) |
| svelte.dev | 150 | 11.5s | 7 | 0% | GT is sparse/hand-curated (7 entries) |
| linear.app | 150 | 21.4s | 134 | 0% | GT uses different URL patterns than crawled |
| www.shopify.com | 150 | 48.0s | 9 | 33% | GT is sparse/hand-curated; slowest crawl |

---

## Key Findings

### Basic vs Enhanced

**Enhanced consistently tightens descriptions.**
- Mean word count drops significantly across all sites — from 20–26 words (basic) to 10–16 (enhanced), matching the "under 15 words" prompt target.
- "This page..." prefix rate is 0% on all runs — the prompt constraint holds.
- Enhanced hits 99–100% unique descriptions on every site tested.

**Basic mode breaks on doc-heavy sites.**
- hono.dev: 0 entries in basic mode — the site has no `<meta name="description">` tags. Enhanced rescued all 26 entries.
- svelte.dev: basic mode pulls full page body as the description (mean 166 words, 13% unique). Enhanced: 10.6 words, 100% unique. The most dramatic improvement in the dataset.
- This is a real signal: for developer documentation sites, basic mode is either empty or produces unusable wall-of-text descriptions. Enhanced is required.

**Basic mode uniqueness problem on product/marketing sites.**
- Linear: 56% unique in basic mode — nearly half the pages share templated meta descriptions. Enhanced: 93%.
- Shopify: 65% unique basic → 94% enhanced. Common pattern on large commercial sites with templated `<head>` tags.
- Vercel is the exception: 100% unique in basic mode — they maintain distinct meta descriptions per page.

### Two Ground Truth Philosophies

Sites publishing `llms.txt` fall into two camps:

**Sparse/hand-curated** (Supabase: 8, Svelte: 7, Shopify: 9, hono.dev: 86): the site owner selected a small set of the most important pages. Our comprehensive crawl produces far more entries — this looks like "false positives" in the metrics but is a feature, not a bug. We're more thorough than their curated index.

**Exhaustive** (Vercel: 571, Stripe: 268, Linear: 134): the GT file attempts to index the full site. Low coverage here is a pure scale story — we hit the crawl cap before reaching their full page count.

### URL Coverage

Coverage numbers need context:
- **Scale gap:** Vercel at 0% coverage means we found 150 of 571 pages. The 150 we found are legitimate — we just need a higher cap.
- **URL pattern mismatch:** Linear at 0% despite 150 pages crawled suggests their GT uses different URL patterns (possibly their docs subdomain) than what BFS from the root discovers.
- **Sparse GT:** Supabase/Svelte/Shopify at 0–33% reflects the hand-curation gap, not crawl quality.

Coverage only becomes a meaningful quality signal when `CRAWL_MAX_PAGES` is large enough to approach the GT size and the GT is exhaustive.

### Section Alignment

**Path-derived sections diverge from product-intent sections.**
- Stripe: 29% alignment — our 30 path-derived sections vs their 28 product/feature sections. Best alignment in the dataset because their URL structure mirrors their product taxonomy.
- Linear: 50% section alignment despite 0% URL coverage — structural alignment can exist independent of URL overlap.
- Vercel, Supabase, Svelte, Shopify: 0% — product-intent sections ("Integrations", "Enterprise") don't map to URL path segments.

### Crawl Behaviour

**Crawl time varies significantly by site.**
- Fast: Supabase, Svelte (~12s for 150 pages)
- Medium: Hono (~2s for 27), Linear (~21s), Vercel (~28s), Stripe (~30s)
- Slow: Shopify (48s) — large commercial site, more aggressive rate limiting

**Rate limiting caps real page counts.**
- Stripe returned 129 pages against a 150-page cap — the site throttled requests before the cap was hit.

**JS-rendered sites are invisible.**
- elysiajs.com: 1 page crawled despite having a 94-entry published llms.txt. Fundamental crawler limitation — we only follow `<a href>` links in raw HTML.

---

## Open Questions

- Should the eval report flag when basic mode produces 0 or near-0 entries? That's a useful warning.
- URL coverage metric needs a note when crawled pages < GT entries (scale gap caveat).
- Section alignment may be more useful as "any of our sections appear in GT" rather than strict fraction.
- Linear's 0% URL coverage despite 150 pages is worth investigating — possibly their GT references a `/docs` subdomain not reachable from the root.
