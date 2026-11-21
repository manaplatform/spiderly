// internal/engine/sort.go
package engine

import (
	"sort"
	"strings"
)

// SortMode defines the sorting strategy for crawl nodes.
type SortMode string

const (
	SortHierarchy SortMode = "hierarchy"
	SortDepth     SortMode = "depth"
	SortCategory  SortMode = "category"
	SortPriceAsc  SortMode = "price"
	SortPriceDesc SortMode = "price-desc"
	SortTitle     SortMode = "title"
	SortDate      SortMode = "date"
	SortRating    SortMode = "rating"
)

// ParseSortMode parses a string into a recognized SortMode, defaulting to SortHierarchy.
func ParseSortMode(s string) SortMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "hierarchy", "tree":
		return SortHierarchy
	case "depth":
		return SortDepth
	case "category", "cat":
		return SortCategory
	case "price", "price-asc", "price_asc":
		return SortPriceAsc
	case "price-desc", "price_desc":
		return SortPriceDesc
	case "title", "name", "alpha":
		return SortTitle
	case "date", "time", "recent":
		return SortDate
	case "rating", "rating-desc":
		return SortRating
	default:
		return SortHierarchy
	}
}

// SortNodes sorts a slice of CrawlNodes according to the specified SortMode.
func SortNodes(nodes []*CrawlNode, mode SortMode, tree *CrawlTree) []*CrawlNode {
	if len(nodes) == 0 {
		return nodes
	}

	result := make([]*CrawlNode, len(nodes))
	copy(result, nodes)

	switch mode {
	case SortHierarchy:
		if tree != nil {
			dfsNodes := tree.FlattenDFS()
			// Preserve only those in the provided nodes slice
			inInput := make(map[string]bool)
			for _, n := range nodes {
				inInput[n.URL] = true
			}
			var filtered []*CrawlNode
			for _, n := range dfsNodes {
				if inInput[n.URL] {
					filtered = append(filtered, n)
				}
			}
			return filtered
		}
		// If no tree, fallback to depth
		sort.SliceStable(result, func(i, j int) bool {
			return result[i].Depth < result[j].Depth
		})

	case SortDepth:
		sort.SliceStable(result, func(i, j int) bool {
			if result[i].Depth != result[j].Depth {
				return result[i].Depth < result[j].Depth
			}
			return result[i].URL < result[j].URL
		})

	case SortCategory:
		sort.SliceStable(result, func(i, j int) bool {
			catI := result[i].Category
			if catI == "" {
				catI = "ZZZ"
			}
			catJ := result[j].Category
			if catJ == "" {
				catJ = "ZZZ"
			}
			if catI != catJ {
				return strings.ToLower(catI) < strings.ToLower(catJ)
			}
			return strings.ToLower(result[i].Title) < strings.ToLower(result[j].Title)
		})

	case SortPriceAsc:
		sort.SliceStable(result, func(i, j int) bool {
			priceI := 0.0
			if result[i].Product != nil {
				priceI = result[i].Product.Price
			}
			priceJ := 0.0
			if result[j].Product != nil {
				priceJ = result[j].Product.Price
			}
			// Non-zero prices come first
			if priceI > 0 && priceJ == 0 {
				return true
			}
			if priceI == 0 && priceJ > 0 {
				return false
			}
			return priceI < priceJ
		})

	case SortPriceDesc:
		sort.SliceStable(result, func(i, j int) bool {
			priceI := 0.0
			if result[i].Product != nil {
				priceI = result[i].Product.Price
			}
			priceJ := 0.0
			if result[j].Product != nil {
				priceJ = result[j].Product.Price
			}
			return priceI > priceJ
		})

	case SortTitle:
		sort.SliceStable(result, func(i, j int) bool {
			return strings.ToLower(result[i].Title) < strings.ToLower(result[j].Title)
		})

	case SortRating:
		sort.SliceStable(result, func(i, j int) bool {
			ratingI := 0.0
			if result[i].Product != nil {
				ratingI = result[i].Product.Rating
			}
			ratingJ := 0.0
			if result[j].Product != nil {
				ratingJ = result[j].Product.Rating
			}
			return ratingI > ratingJ
		})

	case SortDate:
		sort.SliceStable(result, func(i, j int) bool {
			timeI := int64(0)
			if result[i].Page != nil {
				timeI = result[i].Page.ScrapedAt.Unix()
			}
			timeJ := int64(0)
			if result[j].Page != nil {
				timeJ = result[j].Page.ScrapedAt.Unix()
			}
			return timeI > timeJ
		})
	}

	return result
}
