package crawler

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"go.uber.org/zap"
)

// Page is the result of crawling a single URL.
type Page struct {
	URL         string
	Title       string
	Description string
	Body        string
	Depth       int
}

// Config controls crawl behavior. Use NewConfig to construct with defaults applied.
type Config struct {
	maxPages    int
	maxDepth    int
	delay       time.Duration
	concurrency int
}

// NewConfig returns a Config with defaults applied for any zero values:
// maxPages=50, maxDepth=3, delayMS=500, concurrency=1.
func NewConfig(maxPages, maxDepth, delayMS, concurrency int) Config {
	if maxPages == 0 {
		maxPages = 50
	}
	if maxDepth == 0 {
		maxDepth = 3
	}
	if delayMS == 0 {
		delayMS = 500
	}
	if concurrency == 0 {
		concurrency = 1
	}
	return Config{
		maxPages:    maxPages,
		maxDepth:    maxDepth,
		delay:       time.Duration(delayMS) * time.Millisecond,
		concurrency: concurrency,
	}
}

// MaxPages returns the configured page cap.
func (c Config) MaxPages() int { return c.maxPages }

// WithOverrides returns a copy of c with any non-zero values applied.
// Used by the HTTP handler to apply per-request UI overrides.
func (c Config) WithOverrides(maxPages, maxDepth, concurrency int) Config {
	if maxPages > 0 {
		c.maxPages = maxPages
	}
	if maxDepth > 0 {
		c.maxDepth = maxDepth
	}
	if concurrency > 0 {
		c.concurrency = concurrency
	}
	return c
}

type queueItem struct {
	url   string
	depth int
}

// Crawler fetches pages reachable from a given origin.
type Crawler struct {
	cfg    Config
	log    *zap.Logger
	client *http.Client
	// OnPage is called after each page is successfully fetched.
	// crawled is the running total of pages collected so far.
	OnPage func(crawled int)
}

// New creates a Crawler with the given config. If log is nil, debug output is suppressed.
func New(cfg Config, log *zap.Logger) *Crawler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Crawler{
		cfg:    cfg,
		log:    log,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Crawl fetches pages reachable from origin up to the configured depth and
// page limits. It respects robots.txt and uses sitemap.xml to seed the queue
// when available.
func (c *Crawler) Crawl(ctx context.Context, origin string) ([]Page, error) {
	// Resolve the canonical origin by following any redirect (e.g. example.com
	// → www.example.com). All host comparisons use this resolved value.
	if canonical := resolveOrigin(ctx, c.client, origin); canonical != "" {
		origin = canonical
	}

	rb := fetchRobots(ctx, c.client, origin)
	seeds := fetchSitemapURLs(ctx, c.client, origin)

	visited := mapset.NewSet[string]()
	pages := make([]Page, 0, c.cfg.maxPages)

	queue := make([]queueItem, 0, c.cfg.maxPages)
	queue = append(queue, queueItem{url: origin, depth: 0})
	visited.Add(normalizeURL(origin))

	// Seed from sitemap but don't exceed MaxPages entries in the initial queue.
	for _, u := range seeds {
		n := normalizeURL(u)
		if !visited.Contains(n) && sameHost(u, origin) {
			visited.Add(n)
			queue = append(queue, queueItem{url: u, depth: 1})
		}
	}

	c.log.Debug("crawl starting", zap.String("origin", origin), zap.Int("queue_seed", len(queue)))

	type fetchResult struct {
		page  Page
		links []string
		item  queueItem
		ok    bool
	}

	concurrency := c.cfg.concurrency
	maxP := c.cfg.maxPages

	// work is buffered to concurrency — its capacity is the slot gate.
	work := make(chan queueItem, concurrency)
	results := make(chan fetchResult, concurrency)

	// N fixed workers: fetch only, no knowledge of queue or state.
	var wg sync.WaitGroup
	for range concurrency {
		wg.Go(func() {
			for it := range work {
				page, links, ok := fetchPage(ctx, c.client, it.url, it.depth)
				results <- fetchResult{page, links, it, ok}
			}
		})
	}

	// Ticker paces dispatches: delay/concurrency keeps total rate = 1/delay per worker.
	tickInterval := c.cfg.delay
	if concurrency > 1 {
		tickInterval = c.cfg.delay / time.Duration(concurrency)
	}
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	inFlight := 0 // termination counter only; slot gating is via work channel capacity

	for (len(queue) > 0 || inFlight > 0) && len(pages) < maxP {
		if ctx.Err() != nil {
			break
		}

		select {
		case <-ticker.C:
		dispatch:
			for len(queue) > 0 && len(pages)+inFlight < maxP {
				item := queue[0]
				queue = queue[1:]
				if !rb.allowed(item.url) {
					continue
				}
				select {
				case work <- item:
					inFlight++
				default:
					// All worker slots full; put item back for next tick.
					queue = append([]queueItem{item}, queue...)
					break dispatch
				}
			}

		case r := <-results:
			inFlight--
			if !r.ok {
				continue
			}
			pages = append(pages, r.page)
			if c.OnPage != nil {
				c.OnPage(len(pages))
			}
			enqueued := 0
			if r.item.depth < c.cfg.maxDepth {
				for _, link := range r.links {
					if !sameHost(link, origin) || shouldSkip(link) {
						continue
					}
					n := normalizeURL(link)
					if !visited.Contains(n) {
						visited.Add(n)
						queue = append(queue, queueItem{url: link, depth: r.item.depth + 1})
						enqueued++
					}
				}
			}
			c.log.Debug("fetched",
				zap.String("url", r.item.url),
				zap.Int("links_found", len(r.links)),
				zap.Int("enqueued", enqueued),
				zap.Int("queue_len", len(queue)),
				zap.Int("pages_collected", len(pages)),
			)

		case <-ctx.Done():
			close(work)
			wg.Wait()
			return pages, ctx.Err()
		}
	}

	close(work)
	wg.Wait()

	c.log.Info("crawl finished", zap.String("origin", origin), zap.Int("pages_collected", len(pages)))
	return pages, ctx.Err()
}

// fetchPage GETs a URL and extracts its content. Returns false if the page
// should be skipped (non-200, non-HTML, etc.).
func fetchPage(ctx context.Context, client *http.Client, rawURL string, depth int) (Page, []string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Page{}, nil, false
	}
	req.Header.Set("User-Agent", "llms-txt-generator/1.0")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return Page{}, nil, false
	}
	defer resp.Body.Close()

	// Reject pages that redirected off-domain (e.g. go.dev/issue → github.com).
	if resp.Request.URL.Host != req.URL.Host {
		return Page{}, nil, false
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		return Page{}, nil, false
	}

	data := extract(resp.Body, rawURL)

	page := Page{
		URL:         rawURL,
		Title:       data.title,
		Description: data.description,
		Body:        data.body,
		Depth:       depth,
	}
	return page, data.links, true
}

// shouldSkip returns true for URLs that are unlikely to be useful content
// pages (pagination, tag archives, static assets, etc.).
func shouldSkip(u string) bool {
	lower := strings.ToLower(u)
	for _, pat := range skipPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

var skipPatterns = []string{
	// taxonomy / pagination
	"/tag/", "/tags/", "/category/", "/categories/",
	"/page/", "?page=", "&page=",
	// images & styles
	".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".ico",
	".css",
	// scripts & data
	".js", ".json", ".xml", ".wasm",
	// binary downloads
	".pdf", ".zip", ".tar.gz", ".tgz", ".gz", ".msi", ".pkg",
	".exe", ".deb", ".rpm", ".dmg",
	// download/release archive paths
	"/dl/", "/dl",
	// issue trackers
	"/issue/", "/issues/",
	// feeds & CMS internals
	"/feed/", "/rss", "/atom",
	"/wp-content/", "/wp-includes/",
}

// resolveOrigin follows any redirect on the root URL and returns the scheme +
// host of the final destination (e.g. "https://www.example.com"). Returns ""
// on error so the caller can fall back to the original value.
func resolveOrigin(ctx context.Context, client *http.Client, origin string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "llms-txt-generator/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	resp.Body.Close()
	final := resp.Request.URL
	return final.Scheme + "://" + final.Host
}
