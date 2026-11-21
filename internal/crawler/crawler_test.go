// internal/crawler/crawler_test.go
package crawler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCrawler_HeadlessAlways(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>JS Home</title></head>
<body>
<div id="app">Initial</div>
<script>
	document.getElementById('app').innerHTML = '<p>Rendered by ChromeDP</p><a href="/page2">To Page 2</a>';
</script>
</body>
</html>`)
		case "/page2":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Page Two</title></head>
<body>
<p>Welcome to Page 2</p>
</body>
</html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.Headless = true
	cfg.BrowserStrategy = "always"
	cfg.Timeout = 10 * time.Second
	cfg.MaxPages = 5
	cfg.MaxDepth = 2

	crwl, err := NewCrawler(cfg)
	if err != nil {
		t.Skipf("Chrome not available: %v", err)
		return
	}

	results, err := crwl.Crawl(server.URL)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("expected at least 2 pages scraped, got %d", len(results))
	}

	foundHome := false
	foundPage2 := false
	for _, page := range results {
		if strings.Contains(page.Content, "Rendered by ChromeDP") || strings.Contains(page.Title, "JS Home") {
			foundHome = true
		}
		if strings.Contains(page.URL, "/page2") {
			foundPage2 = true
		}
	}

	if !foundHome {
		t.Errorf("expected home page with rendered JS content")
	}
	if !foundPage2 {
		t.Errorf("expected page2 to be discovered from rendered DOM and crawled")
	}
}

func TestCrawler_FallbackStrategy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			// Static page with link to SPA
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Static Root</title></head>
<body>
<h1>Static Content</h1>
<p>This is a plain HTML page that does not require JS.</p>
<a href="/spa">Go to SPA</a>
</body>
</html>`)
		case "/spa":
			// SPA requiring JS
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>SPA App</title></head>
<body>
<noscript>Please enable JavaScript to view this application.</noscript>
<div id="root"></div>
<script>
	document.getElementById('root').innerHTML = '<p>Hydrated SPA Content</p>';
</script>
</body>
</html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.Headless = true
	cfg.BrowserStrategy = "fallback"
	cfg.Timeout = 10 * time.Second
	cfg.MaxPages = 5
	cfg.MaxDepth = 2

	crwl, err := NewCrawler(cfg)
	if err != nil {
		t.Skipf("Chrome not available: %v", err)
		return
	}

	results, err := crwl.Crawl(server.URL)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("expected 2 pages scraped, got %d", len(results))
	}

	foundSPA := false
	for _, page := range results {
		if strings.Contains(page.URL, "/spa") {
			if strings.Contains(page.Content, "Hydrated SPA Content") {
				foundSPA = true
			}
		}
	}

	if !foundSPA {
		t.Errorf("expected /spa page to be re-rendered via ChromeDP fallback and contain hydrated content")
	}
}

func TestCrawler_ClickCrawl(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Interactive Page</title></head>
<body>
<div id="content">
	<button id="more-btn" onclick="addLink()">Show More Links</button>
</div>
<script>
	function addLink() {
		var a = document.createElement('a');
		a.href = '/clicked-link';
		a.textContent = 'Click Target';
		document.getElementById('content').appendChild(a);
	}
</script>
</body>
</html>`)
		case "/clicked-link":
			fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Clicked Link Target</title></head>
<body>
<p>Target page content</p>
</body>
</html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.Headless = true
	cfg.BrowserStrategy = "always"
	cfg.ClickCrawl = true
	cfg.MaxClicksPerPage = 5
	cfg.Timeout = 10 * time.Second
	cfg.MaxPages = 5
	cfg.MaxDepth = 2

	crwl, err := NewCrawler(cfg)
	if err != nil {
		t.Skipf("Chrome not available: %v", err)
		return
	}

	results, err := crwl.Crawl(server.URL)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	foundClickedTarget := false
	for _, page := range results {
		if strings.Contains(page.URL, "/clicked-link") {
			foundClickedTarget = true
		}
	}

	if !foundClickedTarget {
		t.Errorf("expected /clicked-link to be discovered via click-crawl and scraped")
	}
}
