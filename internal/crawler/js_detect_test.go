// internal/crawler/js_detect_test.go
package crawler

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestDetectJSRequired(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		html         string
		clickCrawl   bool
		expectJS     bool
		expectReason string
	}{
		{
			name:       "Normal static HTML",
			statusCode: 200,
			html: `<!DOCTYPE html>
<html>
<head><title>Static Page</title></head>
<body>
<h1>Welcome to our site</h1>
<p>This is a complete static HTML page with lots of rich text and information for visitors to read and explore without needing any JavaScript framework.</p>
</body>
</html>`,
			expectJS: false,
		},
		{
			name:       "Page with noscript tag requiring JS",
			statusCode: 200,
			html: `<!DOCTYPE html>
<html>
<head><title>App</title></head>
<body>
<noscript>You need to enable JavaScript to run this app.</noscript>
<div id="root"></div>
</body>
</html>`,
			expectJS:     true,
			expectReason: "noscript tag requires JavaScript",
		},
		{
			name:       "Empty React root container",
			statusCode: 200,
			html: `<!DOCTYPE html>
<html>
<head><title>React App</title></head>
<body>
<div id="root"></div>
<script src="/bundle.js"></script>
</body>
</html>`,
			expectJS:     true,
			expectReason: "empty SPA container (#root)",
		},
		{
			name:       "Empty Next.js container",
			statusCode: 200,
			html: `<!DOCTYPE html>
<html>
<head><title>Next App</title></head>
<body>
<div id="__next"></div>
<script src="/_next/static/chunks/main.js"></script>
</body>
</html>`,
			expectJS:     true,
			expectReason: "empty SPA container (#__next)",
		},
		{
			name:       "Cloudflare challenge page",
			statusCode: 403,
			html: `<!DOCTYPE html>
<html>
<head><title>Just a moment...</title></head>
<body>
<div id="cf-browser-verification">Checking your browser before accessing example.com</div>
</body>
</html>`,
			expectJS:     true,
			expectReason: "bot protection or JS challenge detected",
		},
		{
			name:       "Minimal body text with scripts",
			statusCode: 200,
			html: `<!DOCTYPE html>
<html>
<head><title>Loading</title></head>
<body>
<p>Loading</p>
<script src="/app.js"></script>
</body>
</html>`,
			expectJS:     true,
			expectReason: "minimal body text with client scripts",
		},
		{
			name:       "Static page with interactive button and click crawl enabled",
			statusCode: 200,
			html: `<!DOCTYPE html>
<html>
<head><title>Button Page</title></head>
<body>
<h1>Page</h1>
<button onclick="reveal()">Click</button>
</body>
</html>`,
			clickCrawl:   true,
			expectJS:     true,
			expectReason: "interactive elements present with click-crawl enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(tt.html))
			if err != nil {
				t.Fatalf("failed to parse HTML: %v", err)
			}

			needed, reason := DetectJSRequired(tt.statusCode, tt.html, doc.Selection, tt.clickCrawl)
			if needed != tt.expectJS {
				t.Errorf("expected DetectJSRequired=%v, got=%v (reason: %s)", tt.expectJS, needed, reason)
			}
			if tt.expectJS && tt.expectReason != "" && !strings.Contains(reason, tt.expectReason) {
				t.Errorf("expected reason to contain %q, got %q", tt.expectReason, reason)
			}
		})
	}
}
