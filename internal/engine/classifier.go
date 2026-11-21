// internal/engine/classifier.go
package engine

import (
	"net/url"
	"strings"

	"spiderly/internal/models"
)

var commonContainers = map[string]bool{
	"category":    true,
	"categories":  true,
	"collection":  true,
	"collections": true,
	"catalog":     true,
	"catalogue":   true,
	"shop":        true,
	"store":       true,
	"products":    true,
	"product":     true,
	"dept":        true,
	"department":  true,
	"c":           true,
	"p":           true,
	"items":       true,
	"sections":    true,
	"section":     true,
}

// ClassifyAndEnrichPage determines the category, subcategory, breadcrumbs,
// and page type for a scraped page using multi-signal heuristics.
func ClassifyAndEnrichPage(p *models.ScrapedPage, parentNode *CrawlNode, rootURL string) {
	if p == nil {
		return
	}

	// 1. Detect Page Type
	if p.PageType == "" {
		p.PageType = detectPageType(p, rootURL)
	}

	// 2. Resolve Breadcrumbs
	if len(p.Breadcrumbs) == 0 {
		if p.StructuredData != nil && len(p.StructuredData.Breadcrumbs) > 0 {
			p.Breadcrumbs = p.StructuredData.Breadcrumbs
		}
	}

	// If still no breadcrumbs, infer from URL path
	if len(p.Breadcrumbs) == 0 {
		p.Breadcrumbs = inferBreadcrumbsFromURL(p.URL)
	}

	// 3. Resolve Category & Subcategory
	category, subcategory := inferCategory(p, parentNode)
	if p.Category == "" && category != "" {
		p.Category = category
	}
	if p.StructuredData != nil && p.StructuredData.Category == "" && p.Category != "" {
		p.StructuredData.Category = p.Category
	}
	if p.Product != nil && p.Product.Category == "" && p.Category != "" {
		p.Product.Category = p.Category
	}

	_ = subcategory
}

func detectPageType(p *models.ScrapedPage, rootURL string) string {
	// Root page check
	if p.Depth == 0 || isRootURL(p.URL, rootURL) {
		return "root"
	}

	// Product page check
	if p.Product != nil && (p.Product.Name != "" || p.Product.Price > 0 || p.Product.SKU != "") {
		return "product"
	}
	if p.StructuredData != nil && (p.StructuredData.Type == "Product" || p.StructuredData.Price > 0) {
		return "product"
	}

	// Article check
	if p.News != nil || (p.StructuredData != nil && p.StructuredData.Type == "Article") {
		return "article"
	}

	// Category check
	parsed, err := url.Parse(p.URL)
	if err == nil {
		path := strings.ToLower(parsed.Path)
		if strings.Contains(path, "/category/") || strings.Contains(path, "/collections/") ||
			strings.Contains(path, "/catalog/") || strings.Contains(path, "/c/") ||
			strings.HasSuffix(path, "/shop") || strings.HasSuffix(path, "/products") {
			return "category"
		}
	}

	return "page"
}

func isRootURL(rawURL, rootURL string) bool {
	u1, err1 := url.Parse(rawURL)
	u2, err2 := url.Parse(rootURL)
	if err1 != nil || err2 != nil {
		return false
	}
	if u1.Host != u2.Host {
		return false
	}
	path1 := strings.Trim(u1.Path, "/")
	path2 := strings.Trim(u2.Path, "/")
	return path1 == path2 || path1 == ""
}

func inferBreadcrumbsFromURL(rawURL string) []string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}

	parts := splitPathSegments(parsed.Path)
	if len(parts) == 0 {
		return []string{"Home"}
	}

	crumbs := []string{"Home"}
	for _, part := range parts {
		if !isIDOrFile(part) {
			clean := formatSegment(part)
			if clean != "" {
				crumbs = append(crumbs, clean)
			}
		}
	}
	return crumbs
}

func inferCategory(p *models.ScrapedPage, parentNode *CrawlNode) (string, string) {
	// Signal 1: Structured Data Category
	if p.StructuredData != nil && p.StructuredData.Category != "" {
		return cleanCategory(p.StructuredData.Category), ""
	}
	if p.Product != nil && p.Product.Category != "" {
		return cleanCategory(p.Product.Category), ""
	}

	// Signal 2: Breadcrumbs
	if len(p.Breadcrumbs) > 1 {
		// Filter out "Home" if it's the first crumb
		var meaningful []string
		for _, b := range p.Breadcrumbs {
			bClean := strings.TrimSpace(b)
			if !strings.EqualFold(bClean, "home") && bClean != "" {
				meaningful = append(meaningful, bClean)
			}
		}
		if len(meaningful) == 1 {
			return meaningful[0], ""
		} else if len(meaningful) >= 2 {
			// e.g. ["Electronics", "Laptops"]
			return meaningful[0], meaningful[1]
		}
	}

	// Signal 3: URL Path Segments
	parsed, err := url.Parse(p.URL)
	if err == nil {
		segments := splitPathSegments(parsed.Path)
		if len(segments) > 0 {
			// Check if first segment is container
			first := strings.ToLower(segments[0])
			if commonContainers[first] && len(segments) > 1 {
				cat := formatSegment(segments[1])
				var subcat string
				if len(segments) > 2 && !isIDOrFile(segments[2]) {
					subcat = formatSegment(segments[2])
				}
				return cat, subcat
			} else if !commonContainers[first] {
				cat := formatSegment(segments[0])
				var subcat string
				if len(segments) > 1 && !isIDOrFile(segments[1]) {
					subcat = formatSegment(segments[1])
				}
				return cat, subcat
			}
		}
	}

	// Signal 4: Parent Node Inheritance
	if parentNode != nil && parentNode.Category != "" {
		return parentNode.Category, ""
	}

	return "General", ""
}

func splitPathSegments(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	var res []string
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s != "" {
			res = append(res, s)
		}
	}
	return res
}

func isIDOrFile(segment string) bool {
	if strings.Contains(segment, ".html") || strings.Contains(segment, ".php") || strings.Contains(segment, ".asp") {
		return true
	}
	// Pure numbers or short hashes
	isNum := true
	for _, r := range segment {
		if r < '0' || r > '9' {
			isNum = false
			break
		}
	}
	return isNum
}

func formatSegment(seg string) string {
	// Strip file extensions
	if idx := strings.LastIndex(seg, "."); idx != -1 {
		seg = seg[:idx]
	}

	seg = strings.ReplaceAll(seg, "-", " ")
	seg = strings.ReplaceAll(seg, "_", " ")
	words := strings.Fields(seg)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}

func cleanCategory(cat string) string {
	cat = strings.TrimSpace(cat)
	if strings.Contains(cat, ">") {
		parts := strings.Split(cat, ">")
		if len(parts) > 0 {
			cat = strings.TrimSpace(parts[0])
		}
	}
	return cat
}
