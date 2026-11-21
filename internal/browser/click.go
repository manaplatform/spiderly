// internal/browser/click.go
package browser

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const clickDiscoveryScript = `
(function(maxClicks) {
	// Guard against blocking dialogs
	window.alert = function() { return true; };
	window.confirm = function() { return true; };
	window.prompt = function() { return null; };
	window.__spiderly_discovered = window.__spiderly_discovered || [];
	window.__spiderly_opened = window.__spiderly_opened || [];
	window.open = function(u) {
		if (u && typeof u === 'string') {
			try {
				window.__spiderly_opened.push(new URL(u, window.location.href).href);
			} catch(e) {}
		}
		return null;
	};

	var currentOrigin = window.location.origin;

	function getAllLinks() {
		var set = new Set();
		var anchors = document.querySelectorAll('a[href]');
		for (var i = 0; i < anchors.length; i++) {
			var href = anchors[i].getAttribute('href');
			if (href && !href.startsWith('javascript:') && href !== '#') {
				try {
					set.add(new URL(href, window.location.href).href);
				} catch(e) {}
			}
		}
		return set;
	}

	var initialLinks = getAllLinks();
	var initialURL = window.location.href;

	// Candidate selectors for interactive navigation / content revelation
	var selectors = [
		'button:not([disabled])',
		'[role="button"]:not([aria-disabled="true"])',
		'a[href="#"]',
		'a[href^="javascript:"]',
		'a:not([href])',
		'[class*="load-more" i]',
		'[class*="loadmore" i]',
		'[class*="pagination" i]',
		'[class*="next-page" i]',
		'[id*="load-more" i]'
	];

	var destructiveRe = /\b(delete|remove|clear|cancel|buy|checkout|cart|add to cart|purchase|pay|sign in|login|log in|logout|sign out|register|subscribe)\b/i;

	var candidates = [];
	for (var s = 0; s < selectors.length; s++) {
		var elements = document.querySelectorAll(selectors[s]);
		for (var e = 0; e < elements.length; e++) {
			var elem = elements[e];

			// Exclude form submit/reset buttons
			var type = (elem.getAttribute('type') || '').toLowerCase();
			if (type === 'submit' || type === 'reset') {
				continue;
			}

			// Exclude destructive actions
			var text = (elem.innerText || elem.textContent || elem.getAttribute('aria-label') || '').trim();
			if (destructiveRe.test(text)) {
				continue;
			}

			// Exclude hidden elements
			if (elem.offsetWidth === 0 && elem.offsetHeight === 0) {
				continue;
			}

			candidates.push(elem);
			if (candidates.length >= maxClicks) {
				break;
			}
		}
		if (candidates.length >= maxClicks) {
			break;
		}
	}

	var discovered = [];

	for (var c = 0; c < candidates.length; c++) {
		var el = candidates[c];
		try {
			el.click();
		} catch (err) {}

		// Check if URL changed (SPA client-side navigation)
		if (window.location.href !== initialURL) {
			discovered.push(window.location.href);
			initialURL = window.location.href;
		}

		// Check if new links were added to the DOM
		var currentLinks = getAllLinks();
		currentLinks.forEach(function(link) {
			if (!initialLinks.has(link)) {
				discovered.push(link);
				initialLinks.add(link);
			}
		});
	}

	// Include any URLs passed to window.open
	if (window.__spiderly_opened && window.__spiderly_opened.length > 0) {
		for (var o = 0; o < window.__spiderly_opened.length; o++) {
			discovered.push(window.__spiderly_opened[o]);
		}
		window.__spiderly_opened = [];
	}

	return Array.from(new Set(discovered));
})(%d);
`

// discoverClicksInTab executes controlled click discovery on the rendered tab.
func (e *Engine) discoverClicksInTab(ctx context.Context, pageURL string, maxClicks int) ([]string, error) {
	if maxClicks <= 0 {
		return nil, nil
	}

	clickCtx, clickCancel := context.WithTimeout(ctx, 8*time.Second)
	defer clickCancel()

	parsedPageURL, err := url.Parse(pageURL)
	if err != nil {
		return nil, err
	}
	pageOrigin := parsedPageURL.Scheme + "://" + parsedPageURL.Host

	script := strings.Replace(clickDiscoveryScript, "%d", string(rune('0'+maxClicks)), 1)
	// For numbers >= 10, properly format
	script = formatClickScript(maxClicks)

	var rawDiscovered []string
	if err := chromedp.Run(clickCtx,
		chromedp.Evaluate(script, &rawDiscovered),
		chromedp.Sleep(200*time.Millisecond),
	); err != nil {
		return nil, err
	}

	// Filter and normalize discovered URLs
	seen := make(map[string]bool)
	validURLs := make([]string, 0, len(rawDiscovered))

	for _, raw := range rawDiscovered {
		raw = strings.TrimSpace(raw)
		if raw == "" || seen[raw] {
			continue
		}
		seen[raw] = true

		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}

		// Only HTTP/HTTPS
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			continue
		}

		// Prefer same origin or relative paths
		discoveredOrigin := parsed.Scheme + "://" + parsed.Host
		if pageOrigin != "" && discoveredOrigin != pageOrigin {
			// Cross-origin link discovered via click
			continue
		}

		validURLs = append(validURLs, parsed.String())
	}

	return validURLs, nil
}

func formatClickScript(maxClicks int) string {
	var numStr string
	if maxClicks <= 0 {
		numStr = "0"
	} else {
		numStr = strings.TrimSpace(strings.ReplaceAll(string(rune(maxClicks)), "\x00", ""))
		if maxClicks >= 10 {
			numStr = fmt.Sprintf("%d", maxClicks)
		} else {
			numStr = fmt.Sprintf("%d", maxClicks)
		}
	}
	return strings.Replace(clickDiscoveryScript, "%d", numStr, 1)
}
