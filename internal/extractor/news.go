package extractor

import (
	"net/url"
	"regexp"
	"strings"

	"spiderly/internal/models"

	"github.com/PuerkitoBio/goquery"
)

var commaSplitRe = regexp.MustCompile(`\s*,\s*`)

// ExtractNews extracts article metadata via HTML/meta tags and DOM parsing.
// It intentionally does not use JSON-LD.
func ExtractNews(doc *goquery.Selection, pageURL string) *models.NewsData {
	news := &models.NewsData{}

	news.Headline = firstNonEmpty(
		cleanWhitespace(doc.Find("h1").First().Text()),
		metaContent(doc, "property", "og:title"),
		metaContent(doc, "name", "twitter:title"),
	)

	news.Author = firstNonEmpty(
		metaContent(doc, "name", "author"),
		metaContent(doc, "property", "article:author"),
		cleanWhitespace(doc.Find("[rel='author']").First().Text()),
		cleanWhitespace(doc.Find(".author, .byline, .article-author").First().Text()),
	)

	news.PublishedDate = firstNonEmpty(
		metaContent(doc, "property", "article:published_time"),
		metaContent(doc, "name", "pubdate"),
		metaContent(doc, "name", "date"),
		attrValue(doc.Find("time[datetime]").First(), "datetime"),
	)

	news.Summary = firstNonEmpty(
		metaContent(doc, "name", "description"),
		metaContent(doc, "property", "og:description"),
		cleanWhitespace(doc.Find("article p").First().Text()),
	)

	tags := collectTagsFromMeta(doc)
	tags = append(tags, collectTagsFromDOM(doc)...)
	tags = append(tags, collectTagsFromURL(pageURL)...)
	news.Tags = dedupTags(tags)

	if news.Headline == "" && news.Author == "" && news.PublishedDate == "" && len(news.Tags) == 0 {
		return nil
	}

	return news
}

func metaContent(doc *goquery.Selection, attrName, attrValue string) string {
	content, ok := doc.Find("meta[" + attrName + "='" + attrValue + "']").Attr("content")
	if !ok {
		return ""
	}
	return cleanWhitespace(content)
}

func attrValue(sel *goquery.Selection, attr string) string {
	if sel == nil {
		return ""
	}
	v, ok := sel.Attr(attr)
	if !ok {
		return ""
	}
	return cleanWhitespace(v)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func collectTagsFromMeta(doc *goquery.Selection) []string {
	tags := make([]string, 0)

	doc.Find("meta[property='article:tag']").Each(func(_ int, s *goquery.Selection) {
		if content, ok := s.Attr("content"); ok {
			tags = append(tags, splitAndCleanTags(content)...)
		}
	})

	for _, key := range []string{"news_keywords", "keywords"} {
		content := metaContent(doc, "name", key)
		if content != "" {
			tags = append(tags, splitAndCleanTags(content)...)
		}
	}

	return tags
}

func collectTagsFromDOM(doc *goquery.Selection) []string {
	tags := make([]string, 0)

	selectors := []string{
		"a[rel='tag']",
		".tags a",
		".tag a",
		".post-tags a",
		".article-tags a",
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
			text := cleanWhitespace(s.Text())
			if text != "" {
				tags = append(tags, text)
			}

			href, ok := s.Attr("href")
			if !ok {
				return
			}
			tags = append(tags, collectTagsFromURL(href)...)
		})
	}

	return tags
}

func collectTagsFromURL(raw string) []string {
	tags := make([]string, 0)
	parsed, err := url.Parse(raw)
	if err != nil {
		return tags
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := 0; i < len(segments)-1; i++ {
		current := strings.ToLower(segments[i])
		if current != "tag" && current != "tags" {
			continue
		}
		next := normalizeTag(segments[i+1])
		if next != "" {
			tags = append(tags, next)
		}
	}

	return tags
}

func splitAndCleanTags(value string) []string {
	parts := commaSplitRe.Split(strings.TrimSpace(value), -1)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := normalizeTag(part)
		if tag != "" {
			out = append(out, tag)
		}
	}
	return out
}

func dedupTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := normalizeTag(raw)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func normalizeTag(value string) string {
	tag := cleanWhitespace(strings.ReplaceAll(value, "-", " "))
	if len(tag) < 2 || len(tag) > 40 {
		return ""
	}
	if strings.Contains(tag, "/") {
		return ""
	}
	return tag
}
