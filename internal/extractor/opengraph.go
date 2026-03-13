// internal/extractor/opengraph.go
package extractor

import (
	"github.com/PuerkitoBio/goquery"
)

// ogData holds extracted Open Graph meta values.
type ogData struct {
	Title       string
	Description string
	Image       string
	SiteName    string
	URL         string
}

// extractFromOpenGraph pulls og: meta tags from the document head.
func extractFromOpenGraph(doc *goquery.Selection) *ogData {
	og := &ogData{}

	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		prop, _ := s.Attr("property")
		content, _ := s.Attr("content")
		if content == "" {
			return
		}

		switch prop {
		case "og:title":
			og.Title = content
		case "og:description":
			og.Description = content
		case "og:image":
			og.Image = content
		case "og:site_name":
			og.SiteName = content
		case "og:url":
			og.URL = content
		}
	})

	// Also check twitter:card meta as fallback
	if og.Title == "" || og.Description == "" || og.Image == "" {
		doc.Find("meta").Each(func(i int, s *goquery.Selection) {
			name, _ := s.Attr("name")
			content, _ := s.Attr("content")
			if content == "" {
				return
			}

			switch name {
			case "twitter:title":
				if og.Title == "" {
					og.Title = content
				}
			case "twitter:description":
				if og.Description == "" {
					og.Description = content
				}
			case "twitter:image":
				if og.Image == "" {
					og.Image = content
				}
			}
		})
	}

	return og
}
