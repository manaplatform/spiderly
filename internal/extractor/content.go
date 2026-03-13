// internal/extractor/content.go
package extractor

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Package-level compiled regex for whitespace normalization.
var whitespaceRe = regexp.MustCompile(`\s+`)

// cleanWhitespace collapses all runs of whitespace into a single space and trims.
func cleanWhitespace(s string) string {
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(s, " "))
}

// extractMainContent extracts the primary text content from a page.
func extractMainContent(doc *goquery.Selection) string {
	clone := doc.Clone()

	// Strip non-content elements
	clone.Find("script, style, nav, footer, header, aside, .sidebar, .menu, .navigation").Remove()

	// Priority order for content containers
	selectors := []string{
		"main",
		"article",
		"[role='main']",
		".content",
		".post-content",
		".entry-content",
		".article-content",
		"#content",
		"#main",
		".main-content",
	}

	var content string
	for _, sel := range selectors {
		if found := clone.Find(sel); found.Length() > 0 {
			content = strings.TrimSpace(found.First().Text())
			if len(content) > 100 {
				break
			}
		}
	}

	// Fallback to body
	if content == "" {
		content = strings.TrimSpace(clone.Find("body").Text())
	}

	content = cleanWhitespace(content)

	// Cap at 50k characters
	if len(content) > 50000 {
		content = content[:50000]
	}

	return content
}
