# llms.txt Generator

A web app that crawls any website and generates an [`llms.txt`](https://llmstxt.org) file — the proposed standard for helping LLMs understand and navigate site content.

For architecture, design decisions, and tradeoffs see [PRESENTATION.md](PRESENTATION.md).

## Setup

**Prerequisites:** Go 1.26+

```bash
git clone https://github.com/noahschumacher/llm-txt
cd llm-txt

cp .env.example .env
# edit .env — at minimum set APP_ENV and APP_PORT

make run
```

Open `http://localhost:8080`.

### Building

```bash
make build
LLM_TXT_ENV_FILE=.env ./bin/llm-txt
```

The binary is fully self-contained — the frontend is embedded via `//go:embed`.

## Configuration

| Variable | Description | Required |
|---|---|---|
| `APP_ENV` | `local` \| `dev` \| `prod` | Yes |
| `APP_PORT` | Port to listen on | Yes |
| `APP_PASSWORD` | Plaintext password gate (default: `profound`) | No |
| `LLM_PROVIDER` | `anthropic` \| `openai` | Enhanced mode only |
| `LLM_API_KEY` | API key for the LLM provider | Enhanced mode only |
| `LLM_MODEL` | Model to use (e.g. `claude-haiku-4-5`, `gpt-4o-mini`) | No |
| `CRAWL_MAX_PAGES` | Max pages to crawl per request (default: `50`) | No |
| `CRAWL_MAX_DEPTH` | Max BFS depth (default: `3`) | No |
| `CRAWL_DELAY_MS` | Delay between requests in ms (default: `500`) | No |
| `CRAWL_CONCURRENCY` | Server-side crawl worker default (default: `1`) | No |
| `LLM_CONCURRENCY` | Max concurrent LLM calls (default: `5`) | No |

Crawl speed can also be set per-request from the UI (Standard / Fast / Turbo), overriding the server default.

## Eval CLI

`tools/eval` scores generated output against heuristics and optionally a site's published `llms.txt`.

```bash
# heuristics only — basic vs enhanced side-by-side
make eval ARGS="--url https://hono.dev"

# + ground truth comparison
make eval ARGS="--url https://hono.dev --ground-truth https://hono.dev/llms.txt"

# + LLM-as-judge (costs money, samples up to 20 overlapping pages)
make eval ARGS="--url https://hono.dev --ground-truth https://hono.dev/llms.txt --llm-judge"

# save report
make eval ARGS="--url https://hono.dev --out tools/eval/_reports/hono.md"
```

See `tools/eval/findings.md` for results from real runs.

## Deployment (Sevalla)

1. Connect your GitHub repo in the Sevalla dashboard
2. Build command: `go build -o bin/llm-txt .`
3. Start command: `./bin/llm-txt`
4. Add environment variables under Settings → Environment Variables

Sevalla handles HTTPS termination — no additional config needed.
