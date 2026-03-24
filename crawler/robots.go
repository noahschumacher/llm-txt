package crawler

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// robots holds the disallow rules for User-agent: *.
type robots struct {
	disallow []string
}

// fetchRobots downloads and parses robots.txt for the given origin.
// Returns an empty robots (allow all) on any fetch or parse error.
func fetchRobots(ctx context.Context, client *http.Client, origin string) robots {
	u := fmt.Sprintf("%s/robots.txt", origin)
	zap.L().Debug("fetching robots.txt", zap.String("url", u))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return robots{}
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return robots{}
	}
	defer resp.Body.Close()

	rb := parseRobots(resp.Body)
	zap.L().Debug("robots.txt parsed", zap.Int("disallow_rules", len(rb.disallow)))
	return rb
}

// parseRobots reads a robots.txt and collects Disallow paths for *.
func parseRobots(r interface{ Read([]byte) (int, error) }) robots {
	var rb robots
	var applicable bool // true while inside a matching User-agent block

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// strip inline comments
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			applicable = false
			continue
		}

		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.ToLower(key))
		val = strings.TrimSpace(val)

		switch key {
		case "user-agent":
			applicable = val == "*"
		case "disallow":
			if applicable && val != "" {
				rb.disallow = append(rb.disallow, val)
			}
		}
	}
	return rb
}

// allowed reports whether the given URL path is permitted by these rules.
func (r robots) allowed(rawURL string) bool {
	for _, prefix := range r.disallow {
		if strings.HasPrefix(pathOf(rawURL), prefix) {
			return false
		}
	}
	return true
}

// pathOf returns just the path portion of rawURL.
func pathOf(rawURL string) string {
	// fast path: find the third slash (after scheme://)
	after := strings.TrimPrefix(rawURL, "https://")
	after = strings.TrimPrefix(after, "http://")
	idx := strings.Index(after, "/")
	if idx < 0 {
		return "/"
	}
	return after[idx:]
}
