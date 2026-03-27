package llm

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// DescribeAll calls Describe concurrently for each page, respecting the
// concurrency limit. Results are returned in the same order as inputs. If any
// call fails, its slot in the output is an empty string (non-fatal).
func DescribeAll(
	ctx context.Context,
	d Describer,
	pages []PageContext,
	concurrency int,
	onDone func(completed int),
	log *zap.Logger,
) []string {
	if concurrency <= 0 {
		concurrency = 5
	}

	var (
		results   = make([]string, len(pages))
		sem       = make(chan struct{}, concurrency)
		mu        sync.Mutex
		wg        sync.WaitGroup
		completed = 0
	)

	for i, page := range pages {
		wg.Go(func() {
			sem <- struct{}{}        // acquire slot — blocks when concurrency limit is reached
			defer func() { <-sem }() // release slot when done

			if page.Body == "" {
				// empty body signals skip (e.g. root page)
			} else if desc, err := d.Describe(ctx, page); err != nil {
				// ctx_err distinguishes a canceled context from an API-level error:
				// nil means the error came from the provider; non-nil means our
				// context was killed and the SDK surfaced it as the error.
				log.Warn("llm describe failed",
					zap.Error(err),
					zap.String("url", page.URL),
					zap.NamedError("ctx_err", ctx.Err()),
				)
			} else {
				results[i] = desc // safe: each goroutine writes its own index
			}

			// snapshot completed count under lock, then call onDone outside it
			// so the callback doesn't hold the lock while doing I/O (SSE flush).
			mu.Lock()
			completed++
			n := completed
			mu.Unlock()

			if onDone != nil {
				onDone(n)
			}
		})
	}

	wg.Wait()
	return results
}
