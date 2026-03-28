---
name: eval
description: This skill should be used when the user asks to "run eval", "evaluate a site", "score the output", "run the eval CLI", "compare against ground truth", or "benchmark against a published llms.txt".
version: 1.0.0
---

Run the llms.txt eval CLI against a site. Construct and execute the right `make eval` command based on what the user provides.

## Steps

1. Determine which flags are needed based on what the user provides (URL only, URL + ground truth, etc.)
2. If the user wants `--llm-judge`, confirm first — it costs money
3. Construct the command and run it
4. Read the output report and summarize the key findings

## Commands

**Heuristics only** (no API key needed for basic):
```bash
make eval ARGS="--url https://example.com"
```

**With ground truth:**
```bash
make eval ARGS="--url https://example.com --ground-truth https://example.com/llms.txt"
```

**With LLM-as-judge** (confirm with user first — costs money):
```bash
make eval ARGS="--url https://example.com --ground-truth https://example.com/llms.txt --llm-judge"
```

**Save report:**
```bash
make eval ARGS="--url https://example.com --ground-truth https://example.com/llms.txt --out tools/eval/_reports/example.md"
```

**Override crawl page limit** (default 50):
```bash
CRAWL_MAX_PAGES=150 make eval ARGS="--url https://example.com --ground-truth https://example.com/llms.txt"
```

## Interpreting results

- **Blank descriptions** — if basic mode produces many blanks, the site has no meta tags; enhanced is required
- **Low unique rate** — templated meta descriptions across pages; enhanced will fix this
- **Low URL coverage** — usually a scale story (crawl cap < GT size), not quality failure
- **Low section alignment** — expected on product sites; their sections reflect intent, ours reflect URL paths
- **False positives** — often legitimate pages the site owner chose not to index

## Known working ground truth sites

| Site | GT URL | GT Entries |
|---|---|---|
| hono.dev | https://hono.dev/llms.txt | 86 |
| stripe.com | https://stripe.com/llms.txt | 268 |
| vercel.com | https://vercel.com/llms.txt | 571 |
| supabase.com | https://supabase.com/llms.txt | 8 |
| svelte.dev | https://svelte.dev/llms.txt | 7 |
| linear.app | https://linear.app/llms.txt | 134 |
| www.shopify.com | https://www.shopify.com/llms.txt | 9 |

See `tools/eval/findings.md` for full results from prior runs.
