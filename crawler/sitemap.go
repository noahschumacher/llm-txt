package crawler

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"
)

// fetchSitemapURLs tries to retrieve URLs from sitemap.xml (or a sitemap
// index). Returns nil on any error so the caller falls back to BFS from root.
func fetchSitemapURLs(ctx context.Context, client *http.Client, origin string) []string {
	u := fmt.Sprintf("%s/sitemap.xml", origin)
	zap.L().Debug("fetching sitemap", zap.String("url", u))
	urls := parseSitemap(ctx, client, u, 0)
	zap.L().Debug("sitemap parsed", zap.Int("urls_found", len(urls)))
	return urls
}

const maxSitemapDepth = 2 // prevent infinite index recursion

func parseSitemap(ctx context.Context, client *http.Client, sitemapURL string, depth int) []string {
	if depth > maxSitemapDepth {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sitemapURL, nil)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	return decodeSitemap(ctx, client, resp.Body, depth)
}

// sitemapIndex covers <sitemapindex> documents.
type sitemapIndex struct {
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

// sitemapURLSet covers <urlset> documents.
type sitemapURLSet struct {
	URLs []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

func decodeSitemap(ctx context.Context, client *http.Client, r io.Reader, depth int) []string {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil
	}

	// Try urlset first (most common).
	var urlset sitemapURLSet
	if err := xml.Unmarshal(data, &urlset); err == nil && len(urlset.URLs) > 0 {
		out := make([]string, 0, len(urlset.URLs))
		for _, u := range urlset.URLs {
			if u.Loc != "" {
				out = append(out, u.Loc)
			}
		}
		return out
	}

	// Fall back to sitemap index.
	var idx sitemapIndex
	if err := xml.Unmarshal(data, &idx); err == nil {
		out := make([]string, 0, len(idx.Sitemaps))
		for _, s := range idx.Sitemaps {
			out = append(out, parseSitemap(ctx, client, s.Loc, depth+1)...)
		}
		return out
	}

	return nil
}
