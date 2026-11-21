// internal/engine/filter.go
package engine

import (
	"strings"

	"spiderly/internal/models"
)

// FilterOptions specifies criteria for filtering crawled pages.
type FilterOptions struct {
	Category        string
	MinPrice        *float64
	MaxPrice        *float64
	Availability    string
	InStockOnly     bool
	HasProduct      bool
	Keywords        []string
	MinDepth        *int
	MaxDepth        *int
	PageTypes       []string
	CustomPredicate func(node *CrawlNode) bool
}

// Matches checks whether a node passes all configured filter options.
func (opts FilterOptions) Matches(n *CrawlNode) bool {
	if n == nil {
		return false
	}

	// 1. Category filter
	if opts.Category != "" {
		target := strings.ToLower(opts.Category)
		catMatch := strings.Contains(strings.ToLower(n.Category), target)
		if !catMatch {
			// check breadcrumbs
			foundInCrumbs := false
			for _, b := range n.Breadcrumbs {
				if strings.Contains(strings.ToLower(b), target) {
					foundInCrumbs = true
					break
				}
			}
			if !foundInCrumbs {
				return false
			}
		}
	}

	// 2. HasProduct filter
	if opts.HasProduct {
		if n.Product == nil || (n.Product.Name == "" && n.Product.Price == 0 && n.Product.SKU == "") {
			return false
		}
	}

	// 3. Price range filter
	if opts.MinPrice != nil {
		if n.Product == nil || n.Product.Price < *opts.MinPrice {
			return false
		}
	}
	if opts.MaxPrice != nil {
		if n.Product == nil || n.Product.Price > *opts.MaxPrice {
			return false
		}
	}

	// 4. InStock / Availability
	if opts.InStockOnly {
		if n.Product == nil || !n.Product.InStock {
			return false
		}
	}
	if opts.Availability != "" {
		if n.Product == nil || !strings.EqualFold(n.Product.Availability, opts.Availability) {
			return false
		}
	}

	// 5. Depth filter
	if opts.MinDepth != nil && n.Depth < *opts.MinDepth {
		return false
	}
	if opts.MaxDepth != nil && n.Depth > *opts.MaxDepth {
		return false
	}

	// 6. PageType filter
	if len(opts.PageTypes) > 0 {
		match := false
		for _, pt := range opts.PageTypes {
			if strings.EqualFold(n.PageType, pt) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	// 7. Keywords filter
	if len(opts.Keywords) > 0 {
		for _, kw := range opts.Keywords {
			kw = strings.ToLower(kw)
			found := strings.Contains(strings.ToLower(n.Title), kw) ||
				strings.Contains(strings.ToLower(n.URL), kw) ||
				strings.Contains(strings.ToLower(n.Category), kw)
			if !found && n.Product != nil {
				found = strings.Contains(strings.ToLower(n.Product.Name), kw) ||
					strings.Contains(strings.ToLower(n.Product.Brand), kw) ||
					strings.Contains(strings.ToLower(n.Product.Description), kw) ||
					strings.Contains(strings.ToLower(n.Product.SKU), kw)
			}
			if !found && n.Page != nil {
				found = strings.Contains(strings.ToLower(n.Page.Description), kw) ||
					strings.Contains(strings.ToLower(n.Page.H1), kw)
			}
			if !found {
				return false
			}
		}
	}

	// 8. Custom predicate
	if opts.CustomPredicate != nil && !opts.CustomPredicate(n) {
		return false
	}

	return true
}

// FilterNodes applies FilterOptions to a slice of CrawlNodes.
func FilterNodes(nodes []*CrawlNode, opts FilterOptions) []*CrawlNode {
	var matched []*CrawlNode
	for _, n := range nodes {
		if opts.Matches(n) {
			matched = append(matched, n)
		}
	}
	return matched
}

// FilterTree applies FilterOptions to an entire CrawlTree and returns a new filtered tree.
func FilterTree(tree *CrawlTree, opts FilterOptions) *CrawlTree {
	if tree == nil {
		return nil
	}

	var matchingPages []models.ScrapedPage
	for _, n := range tree.Nodes {
		if opts.Matches(n) && n.Page != nil {
			matchingPages = append(matchingPages, *n.Page)
		}
	}

	return BuildTree(matchingPages, tree.RootURL)
}
