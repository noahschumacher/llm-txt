# Evaluation Framework

## Status: Built ✓

Implemented in `tools/eval/`. See `tools/eval/findings.md` for results from actual runs.

## Usage

```bash
# heuristics only (basic vs enhanced, no LLM key needed for basic)
make eval ARGS="--url https://example.com"

# + ground truth comparison
make eval ARGS="--url https://example.com --ground-truth https://example.com/llms.txt"

# + LLM-as-judge (costs money, samples up to 20 pages)
make eval ARGS="--url https://example.com --ground-truth https://example.com/llms.txt --llm-judge"

# save report to file
make eval ARGS="--url https://example.com --out tools/eval/_reports/example.md"
```

## Two Eval Modes

### 1. Basic vs Enhanced Comparison

Runs both modes on the same crawl and scores them side-by-side:

| Dimension | Method |
|---|---|
| Mean description words | avg word count — shorter is better up to a point |
| "This page..." prefix rate | regex — lower is better |
| Blank descriptions | count — 0 is ideal |
| Unique descriptions | % of non-blank descriptions that are distinct |
| Section count | more sections = better content organisation |

Runs entirely offline — no LLM needed, pure string analysis.

### 2. Ground-Truth Comparison

Fetches a site's published `llms.txt` and scores our output against it:

| Dimension | Method |
|---|---|
| URL coverage | % of GT URLs present in our output |
| Section alignment | % of GT sections matched by ours |
| False positives | URLs we include that they don't |

Optional `--llm-judge` flag samples up to 20 overlapping pages and asks the LLM: given this page body, which description is more useful — ours or theirs?

## Known Working Ground Truth Sites

| Site | GT Entries | Notes |
|---|---|---|
| `https://stripe.com/llms.txt` | 268 | Standard format; site crawls fine. Coverage low at 50-page cap — raise `CRAWL_MAX_PAGES` |
| `https://hono.dev/llms.txt` | 86 | Standard format; 27 pages crawlable. Good size for 50-page runs |

**Sites that don't work:**
- `https://anthropic.com/llms.txt` — 404; site blocks crawler (0 pages crawled)
- `https://fly.io/llms.txt` — 404
- `https://docs.github.com/llms.txt` — returns API endpoint index, not page URLs
- `https://elysiajs.com/llms.txt` — JS-rendered, only 1 page crawlable

Ground truth comparison works against any site's llms.txt regardless of format quality — imperfect format just lowers scores, which is itself a signal.

## Key Learnings from Runs

See `tools/eval/findings.md` for full detail. Headline findings:

- **Enhanced is not optional on doc sites.** Sites without `<meta name="description">` (e.g. hono.dev) produce 0 entries in basic mode; enhanced rescues all of them.
- **Coverage numbers are a scale story.** Low URL coverage usually means crawl cap < GT size, not quality failure.
- **Section alignment is structurally low.** Our sections derive from URL path segments; GT sections reflect product intent. These diverge on marketing sites.
- **JS-rendered sites are invisible.** We only follow `<a href>` in raw HTML. Sites requiring JS for navigation can't be crawled.
- **False positives are low.** On hono.dev: 1/26 pages we found weren't in their GT. On stripe.com: 41 were marketing pages they intentionally excluded.

## Notes

- LLM-as-judge calls cost money — gate behind `--llm-judge` flag, run manually
- Heuristic scores are free and fast — can run in CI against a fixed set of URLs
- Even without ground truth, basic vs enhanced comparison on a fixed corpus gives regression signal when the prompt changes
