# CLAUDE.md

Development guide for the llms.txt Generator.

## Commands

```bash
make run      # run locally (requires .env)
make build    # build to bin/llm-txt
make test     # run all tests
make tidy     # go mod tidy && go mod vendor

# Add a dependency — import it in code first, then run:
make tidy        # resolves, downloads, and vendors the package
```

## Project Structure

```
llm-txt/
├── main.go                      # env loading, dependency wiring, graceful shutdown
├── server/
│   ├── server.go                # server struct, route registration
│   ├── generate.go              # /generate handler, SSE streaming
│   ├── middleware/              # request-scoped middleware (log, timeout)
│   └── services/
│       └── generator/           # crawl → describe → format pipeline
├── crawler/                     # BFS crawler, robots.txt, sitemap, HTML extraction
├── clients/
│   └── llm/                     # LLM API client and concurrent worker pool
├── pkg/
│   └── env.go                   # env var loading helpers
├── static/
│   └── index.html               # embedded single-page frontend
└── vendor/                      # all dependencies are vendored
```

## Architecture

- Single Go binary — frontend embedded via `//go:embed static`
- `server/` owns the HTTP layer: server struct, route registration, handlers (one file per domain), middleware, and services
- Handlers handle HTTP concerns only (decode, validate, encode). Business logic lives in `server/services/`
- External API clients live in `clients/`
- `pkg/` is for shared utilities with no business logic

## Code Patterns

### Handler

```go
func (s *Server) handleFoo(w http.ResponseWriter, r *http.Request) {
    var req fooRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }

    result, err := s.fooService.DoThing(r.Context(), req)
    if err != nil {
        // log here because the error stops at the HTTP boundary — it won't
        // be returned further up the call stack.
        s.log.Error("foo failed", zap.Error(err))
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}
```

### Service

```go
type Service struct {
    log *zap.Logger
    // dependencies
}

func NewService(log *zap.Logger) *Service {
    return &Service{log: log}
}

func (s *Service) DoThing(ctx context.Context, ...) (Result, error) {
    // business logic
}
```

### Adding a Route

Register in `server/server.go` `ListenAndServe`. Apply `mw.TimeoutMiddleware` for normal routes; omit it for SSE endpoints (long-lived connections).

```go
s.router.With(mw.TimeoutMiddleware(15 * time.Second)).Get("/foo", s.handleFoo)
s.router.Post("/stream", s.handleStream) // no timeout
```

## Style Rules

- **Exported identifiers:** only export what must be public. Default to unexported.
- **Error handling:** return errors upward; only log an error when it is not also
  being returned. Never silently ignore.
- **Logging:** use `zap` structured fields (`zap.String`, `zap.Error`, etc.).
  Never `fmt.Println`.
- **Comments:** only where the logic isn't self-evident. Wrap at 80 characters.
  Inline field comments (same line) may exceed 80 characters.
- **No `//nolint` directives.**
- **Context:** pass `context.Context` as the first argument to any I/O operation.
- **Naming:** Go conventions — `userID` not `user_id`, `llmClient` not `LLMClient`.
- **Dependencies:** always run `go mod tidy && go mod vendor` after adding/removing
  packages.
- **Slices:** always initialize with `make([]T, 0)`, never `[]T{}`. Pass a
  capacity hint whenever it's known ahead of time
  (e.g. `make([]T, 0, len(input))`). This avoids unnecessary reallocations
  and makes the intended size explicit at the call site.
- **Maps:** always initialize with `make(map[K]V)`, never `map[K]V{}`. Pass
  a size hint whenever it's known (e.g. `make(map[K]V, len(input))`). Maps
  are especially important to pre-size — a zero-hint map rehashes
  aggressively as it grows, which is more expensive than slice reallocation.

## Environment

Loaded in `main.go` via `pkg.LoadStringEnv`, `pkg.LoadIntEnv`, etc. Required vars fatal on startup if unset. Optional vars return zero values when empty. See `.env.example` for all variables.
