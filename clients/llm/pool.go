package llm

import (
	"context"
	"sync"
)

// DescribeAll calls Describe concurrently for each body, respecting the
// concurrency limit. Results are returned in the same order as inputs. If any
// call fails, its slot in the output is an empty string (non-fatal).
func DescribeAll(ctx context.Context, d Describer, bodies []string, concurrency int, onDone func(completed int)) []string {
	if concurrency <= 0 {
		concurrency = 5
	}

	var (
		results   = make([]string, len(bodies))
		sem       = make(chan struct{}, concurrency)
		mu        sync.Mutex
		wg        sync.WaitGroup
		completed = 0
	)

	for i, body := range bodies {
		wg.Go(func() {
			sem <- struct{}{}        // acquire slot — blocks when concurrency limit is reached
			defer func() { <-sem }() // release slot when done

			desc, err := d.Describe(ctx, body)
			if err == nil {
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
