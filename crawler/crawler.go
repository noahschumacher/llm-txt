package crawler

import (
	"context"
	"net/http"
	"strings"
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

// Config controls crawl behavior. Zero values apply the defaults shown below.
type Config struct {
	MaxPages int         // default 50
	MaxDepth int         // default 3
	DelayMS  int         // default 500
	Log      *zap.Logger // optional; debug logs suppressed if nil
}

func (c Config) log() *zap.Logger {
	if c.Log != nil {
		return c.Log
	}
	return zap.NewNop()
}

func (c Config) maxPages() int {
	if c.MaxPages > 0 {
		return c.MaxPages
	}
	return 50
}

func (c Config) maxDepth() int {
	if c.MaxDepth > 0 {
		return c.MaxDepth
	}
	return 3
}

func (c Config) delay() time.Duration {
	if c.DelayMS > 0 {
		return time.Duration(c.DelayMS) * time.Millisecond
	}
	return 500 * time.Millisecond
}

type queueItem struct {
	url   string
	depth int
}

// Crawl fetches pages reachable from origin up to the configured depth and
// page limits. It respects robots.txt and uses sitemap.xml to seed the queue
// when available.
func Crawl(ctx context.Context, origin string, cfg Config) ([]Page, error) {
	log := cfg.log()
	client := &http.Client{Timeout: 15 * time.Second}

	rb := fetchRobots(ctx, client, origin)
	seeds := fetchSitemapURLs(ctx, client, origin)

	visited := mapset.NewSet[string]()
	pages := make([]Page, 0, cfg.maxPages())

	queue := make([]queueItem, 0, cfg.maxPages())
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

	log.Debug("crawl starting", zap.String("origin", origin), zap.Int("queue_seed", len(queue)))

	for len(queue) > 0 && len(pages) < cfg.maxPages() {
		if ctx.Err() != nil {
			return pages, ctx.Err()
		}

		item := queue[0]
		queue = queue[1:]

		if !rb.allowed(item.url) {
			log.Debug("skipped (robots)", zap.String("url", item.url))
			continue
		}

		log.Debug("fetching", zap.String("url", item.url), zap.Int("depth", item.depth))

		page, links, ok := fetchPage(ctx, client, item.url, item.depth)
		if !ok {
			log.Debug("skipped (non-html or error)", zap.String("url", item.url))
			continue
		}
		pages = append(pages, page)

		// Enqueue new links if we haven't hit depth/page limits yet.
		enqueued := 0
		if item.depth < cfg.maxDepth() {
			for _, link := range links {
				if !sameHost(link, origin) {
					continue
				}
				if shouldSkip(link) {
					continue
				}
				n := normalizeURL(link)
				if !visited.Contains(n) {
					visited.Add(n)
					queue = append(queue, queueItem{url: link, depth: item.depth + 1})
					enqueued++
				}
			}
		}

		log.Debug("fetched",
			zap.String("url", item.url),
			zap.Int("links_found", len(links)),
			zap.Int("enqueued", enqueued),
			zap.Int("queue_len", len(queue)),
			zap.Int("pages_collected", len(pages)),
		)

		if len(queue) > 0 {
			time.Sleep(cfg.delay())
		}
	}

	log.Debug("crawl complete", zap.String("origin", origin), zap.Int("pages", len(pages)))
	return pages, nil
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
