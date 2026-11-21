// internal/browser/browser_test.go
package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBrowserRender_JSExecution(t *testing.T) {
	// Start a test server serving dynamic JS content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>JS Test Page</title></head>
<body>
<div id="app">Loading...</div>
<script>
	setTimeout(function() {
		document.getElementById('app').innerHTML = '<h1>Dynamic Content Loaded</h1><p>Client rendered.</p>';
		document.title = 'Rendered Title';
	}, 100);
</script>
</body>
</html>`)
	}))
	defer server.Close()

	cfg := Config{
		Headless:    true,
		Timeout:     10 * time.Second,
		Concurrency: 2,
	}

	engine, err := NewEngine(cfg)
	if err != nil {
		t.Skipf("skipping test: Chrome not available: %v", err)
		return
	}
	defer engine.Close()

	res, err := engine.Render(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if res.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}
	if !strings.Contains(res.HTML, "Dynamic Content Loaded") {
		t.Errorf("expected rendered HTML to contain 'Dynamic Content Loaded', got:\n%s", res.HTML)
	}
	if res.Title != "Rendered Title" && res.Title != "JS Test Page" {
		t.Errorf("unexpected title: %q", res.Title)
	}
}

func TestBrowserRender_ClickDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Click Discovery Test</title></head>
<body>
<div id="content">
	<button id="load-more-btn" onclick="revealLinks()">Load More</button>
</div>
<script>
	function revealLinks() {
		var div = document.createElement('div');
		div.innerHTML = '<a href="/revealed-item-1">Item 1</a><a href="/revealed-item-2">Item 2</a>';
		document.getElementById('content').appendChild(div);
	}
</script>
</body>
</html>`)
	}))
	defer server.Close()

	cfg := Config{
		Headless:         true,
		Timeout:          10 * time.Second,
		Concurrency:      1,
		ClickCrawl:       true,
		MaxClicksPerPage: 5,
	}

	engine, err := NewEngine(cfg)
	if err != nil {
		t.Skipf("skipping test: Chrome not available: %v", err)
		return
	}
	defer engine.Close()

	res, err := engine.Render(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	foundItem1 := false
	foundItem2 := false
	for _, link := range res.DiscoveredLinks {
		if strings.Contains(link, "/revealed-item-1") {
			foundItem1 = true
		}
		if strings.Contains(link, "/revealed-item-2") {
			foundItem2 = true
		}
	}

	if !foundItem1 || !foundItem2 {
		t.Errorf("expected click discovery to find /revealed-item-1 and /revealed-item-2, got: %v", res.DiscoveredLinks)
	}
}
