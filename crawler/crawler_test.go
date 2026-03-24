package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---- robots ----------------------------------------------------------------

func TestRobotsAllowed(t *testing.T) {
	cases := []struct {
		name    string
		txt     string
		url     string
		allowed bool
	}{
		{
			name:    "no rules allows all",
			txt:     "",
			url:     "https://example.com/anything",
			allowed: true,
		},
		{
			name:    "disallowed prefix blocked",
			txt:     "User-agent: *\nDisallow: /private/",
			url:     "https://example.com/private/secret",
			allowed: false,
		},
		{
			name:    "other agent rule ignored",
			txt:     "User-agent: Googlebot\nDisallow: /private/",
			url:     "https://example.com/private/secret",
			allowed: true,
		},
		{
			name:    "disallow root blocks everything",
			txt:     "User-agent: *\nDisallow: /",
			url:     "https://example.com/page",
			allowed: false,
		},
		{
			name:    "non-matching path allowed",
			txt:     "User-agent: *\nDisallow: /admin/",
			url:     "https://example.com/about",
			allowed: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rb := parseRobots(strings.NewReader(tc.txt))
			if got := rb.allowed(tc.url); got != tc.allowed {
				t.Errorf("allowed(%q) = %v, want %v", tc.url, got, tc.allowed)
			}
		})
	}
}

// ---- extractor -------------------------------------------------------------

func TestExtract(t *testing.T) {
	htmlDoc := `<!DOCTYPE html>
<html>
<head>
  <title>  Hello World  </title>
  <meta name="description" content="A test page.">
</head>
<body>
  <h1>Welcome</h1>
  <p>Some <a href="/about">about</a> content.</p>
  <script>var x = 1;</script>
</body>
</html>`

	data := extract(strings.NewReader(htmlDoc), "https://example.com/")

	if data.title != "Hello World" {
		t.Errorf("title = %q, want %q", data.title, "Hello World")
	}
	if data.description != "A test page." {
		t.Errorf("description = %q, want %q", data.description, "A test page.")
	}
	if !strings.Contains(data.body, "Welcome") {
		t.Errorf("body missing h1 text: %q", data.body)
	}
	if strings.Contains(data.body, "var x") {
		t.Errorf("body should not contain script text: %q", data.body)
	}
	if len(data.links) == 0 {
		t.Fatal("expected at least one link")
	}
	if data.links[0] != "https://example.com/about" {
		t.Errorf("link = %q, want %q", data.links[0], "https://example.com/about")
	}
}

// ---- url helpers -----------------------------------------------------------

func TestResolveLink(t *testing.T) {
	cases := []struct {
		href    string
		base    string
		want    string
	}{
		{"/about", "https://example.com", "https://example.com/about"},
		{"https://other.com/page", "https://example.com", "https://other.com/page"},
		{"#section", "https://example.com/page", ""},          // fragment-only
		{"mailto:foo@bar.com", "https://example.com", ""},     // non-http
		{"page?q=1#frag", "https://example.com/", "https://example.com/page?q=1"}, // fragment stripped
	}
	for _, tc := range cases {
		got := resolveLink(tc.href, tc.base)
		if got != tc.want {
			t.Errorf("resolveLink(%q, %q) = %q, want %q", tc.href, tc.base, got, tc.want)
		}
	}
}

func TestSameHost(t *testing.T) {
	if !sameHost("https://example.com/page", "https://example.com") {
		t.Error("expected same host")
	}
	if sameHost("https://other.com/page", "https://example.com") {
		t.Error("expected different host")
	}
}

// ---- sitemap ---------------------------------------------------------------

func TestParseSitemap_URLSet(t *testing.T) {
	body := `<?xml version="1.0"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/page1</loc></url>
  <url><loc>https://example.com/page2</loc></url>
</urlset>`

	urls := decodeSitemap(context.Background(), &http.Client{}, strings.NewReader(body), 0)
	if len(urls) != 2 {
		t.Fatalf("got %d URLs, want 2", len(urls))
	}
	if urls[0] != "https://example.com/page1" {
		t.Errorf("urls[0] = %q", urls[0])
	}
}

func TestParseSitemap_Index(t *testing.T) {
	sub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/a</loc></url>
</urlset>`))
	}))
	defer sub.Close()

	index := `<?xml version="1.0"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>` + sub.URL + `</loc></sitemap>
</sitemapindex>`

	client := sub.Client()
	urls := decodeSitemap(context.Background(), client, strings.NewReader(index), 0)
	if len(urls) != 1 || urls[0] != "https://example.com/a" {
		t.Errorf("got %v", urls)
	}
}

// ---- crawler integration ---------------------------------------------------

func TestCrawl_BasicBFS(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Home</title><meta name="description" content="home page"></head>
<body><a href="/about">About</a></body></html>`))
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>About</title></head><body>About us.</body></html>`))
	})
	// robots.txt and sitemap.xml — return 404 to test fallback BFS
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := Config{MaxPages: 10, MaxDepth: 2, DelayMS: 0}
	pages, err := Crawl(context.Background(), srv.URL, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) < 2 {
		t.Fatalf("expected at least 2 pages, got %d", len(pages))
	}

	titles := map[string]bool{}
	for _, p := range pages {
		titles[p.Title] = true
	}
	if !titles["Home"] || !titles["About"] {
		t.Errorf("expected Home and About pages, got titles: %v", titles)
	}
}

func TestCrawl_RespectsRobots(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("User-agent: *\nDisallow: /secret/\n"))
	})
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Home</title></head>
<body><a href="/secret/data">secret</a></body></html>`))
	})
	mux.HandleFunc("/secret/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Secret</title></head><body>secret</body></html>`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := Config{MaxPages: 10, MaxDepth: 2, DelayMS: 0}
	pages, err := Crawl(context.Background(), srv.URL, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, p := range pages {
		if strings.Contains(p.URL, "/secret/") {
			t.Errorf("crawled disallowed URL: %s", p.URL)
		}
	}
}

func TestCrawl_MaxPages(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	// Home links to many pages
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		body := `<html><head><title>Home</title></head><body>`
		for i := 1; i <= 20; i++ {
			body += `<a href="/page` + string(rune('0'+i)) + `">p</a>`
		}
		body += `</body></html>`
		w.Write([]byte(body))
	})
	for i := 1; i <= 20; i++ {
		path := "/page" + string(rune('0'+i))
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><title>Page</title></head><body>content</body></html>`))
		})
	}

	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := Config{MaxPages: 3, MaxDepth: 3, DelayMS: 0}
	pages, err := Crawl(context.Background(), srv.URL, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) > cfg.MaxPages {
		t.Errorf("got %d pages, want <= %d", len(pages), cfg.MaxPages)
	}
}
