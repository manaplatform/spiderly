// internal/engine/tree.go
package engine

import (
	"fmt"
	"sort"
	"strings"

	"spiderly/internal/models"
)

// CrawlNode represents a node in the website crawl hierarchy tree.
type CrawlNode struct {
	URL         string                 `json:"url"`
	ParentURL   string                 `json:"parent_url,omitempty"`
	Depth       int                    `json:"depth"`
	Title       string                 `json:"title"`
	StatusCode  int                    `json:"status_code"`
	Category    string                 `json:"category,omitempty"`
	Subcategory string                 `json:"subcategory,omitempty"`
	Breadcrumbs []string               `json:"breadcrumbs,omitempty"`
	PageType    string                 `json:"page_type,omitempty"`
	Product     *models.ProductInfo    `json:"product,omitempty"`
	Structured  *models.StructuredData `json:"structured_data,omitempty"`
	Children    []*CrawlNode           `json:"children,omitempty"`
	Page        *models.ScrapedPage    `json:"-"`
}

// CategorySummary provides aggregate metrics for a category.
type CategorySummary struct {
	Name         string  `json:"name"`
	TotalPages   int     `json:"total_pages"`
	ProductCount int     `json:"product_count"`
	MinPrice     float64 `json:"min_price,omitempty"`
	MaxPrice     float64 `json:"max_price,omitempty"`
	AvgPrice     float64 `json:"avg_price,omitempty"`
}

// ProductSummary provides aggregate metrics across all extracted products.
type ProductSummary struct {
	TotalProducts   int      `json:"total_products"`
	InStockCount    int      `json:"in_stock_count"`
	OutOfStockCount int      `json:"out_of_stock_count"`
	MinPrice        float64  `json:"min_price"`
	MaxPrice        float64  `json:"max_price"`
	AvgPrice        float64  `json:"avg_price"`
	Currencies      []string `json:"currencies,omitempty"`
}

// CrawlTree maintains the full site hierarchy, parent-child graph, and indices.
type CrawlTree struct {
	RootURL    string                  `json:"root_url"`
	Root       *CrawlNode              `json:"root"`
	Nodes      map[string]*CrawlNode   `json:"-"`
	ByCategory map[string][]*CrawlNode `json:"-"`
	ByParent   map[string][]*CrawlNode `json:"-"`
	ByDepth    map[int][]*CrawlNode    `json:"-"`
}

// BuildTree constructs a CrawlTree from a list of scraped pages.
func BuildTree(pages []models.ScrapedPage, rootURL string) *CrawlTree {
	tree := &CrawlTree{
		RootURL:    rootURL,
		Nodes:      make(map[string]*CrawlNode),
		ByCategory: make(map[string][]*CrawlNode),
		ByParent:   make(map[string][]*CrawlNode),
		ByDepth:    make(map[int][]*CrawlNode),
	}

	if len(pages) == 0 {
		return tree
	}

	// 1. First pass: Create all nodes
	var rootCandidates []*CrawlNode
	for i := range pages {
		p := &pages[i]
		node := &CrawlNode{
			URL:        p.URL,
			ParentURL:  p.ParentURL,
			Depth:      p.Depth,
			Title:      p.Title,
			StatusCode: p.StatusCode,
			PageType:   p.PageType,
			Product:    p.Product,
			Structured: p.StructuredData,
			Page:       p,
			Children:   make([]*CrawlNode, 0),
		}

		// Save in map
		tree.Nodes[node.URL] = node

		if node.Depth == 0 || (rootURL != "" && strings.TrimRight(node.URL, "/") == strings.TrimRight(rootURL, "/")) {
			rootCandidates = append(rootCandidates, node)
		}
	}

	// 2. Identify the true Root Node
	if len(rootCandidates) > 0 {
		tree.Root = rootCandidates[0]
	} else if len(pages) > 0 {
		// Lowest depth node as root
		var lowestNode *CrawlNode
		for _, n := range tree.Nodes {
			if lowestNode == nil || n.Depth < lowestNode.Depth {
				lowestNode = n
			}
		}
		tree.Root = lowestNode
	}

	if tree.Root != nil && tree.RootURL == "" {
		tree.RootURL = tree.Root.URL
	}

	// 3. Second pass: Classify and Enrich nodes (respecting parent lineage)
	for _, node := range tree.Nodes {
		var parentNode *CrawlNode
		if node.ParentURL != "" {
			parentNode = tree.Nodes[node.ParentURL]
		}
		if node.Page != nil {
			ClassifyAndEnrichPage(node.Page, parentNode, tree.RootURL)
			node.Category = node.Page.Category
			node.Breadcrumbs = node.Page.Breadcrumbs
			node.PageType = node.Page.PageType
			node.Product = node.Page.Product
			node.Structured = node.Page.StructuredData
		}
	}

	// 4. Third pass: Link parent-child relationships and prevent cycles
	for _, node := range tree.Nodes {
		if node == tree.Root {
			continue
		}

		if node.ParentURL != "" && tree.Nodes[node.ParentURL] != nil {
			parent := tree.Nodes[node.ParentURL]
			if !hasCycle(node, parent) {
				parent.Children = append(parent.Children, node)
				tree.ByParent[node.ParentURL] = append(tree.ByParent[node.ParentURL], node)
				continue
			}
		}

		// If no parent found or cycle prevented, attach to Root if available
		if tree.Root != nil && node != tree.Root {
			tree.Root.Children = append(tree.Root.Children, node)
			tree.ByParent[tree.Root.URL] = append(tree.ByParent[tree.Root.URL], node)
		}
	}

	// 5. Index by Category and Depth, and sort children
	for _, node := range tree.Nodes {
		cat := node.Category
		if cat == "" {
			cat = "General"
		}
		tree.ByCategory[cat] = append(tree.ByCategory[cat], node)
		tree.ByDepth[node.Depth] = append(tree.ByDepth[node.Depth], node)

		// Sort children by depth then URL
		sort.SliceStable(node.Children, func(i, j int) bool {
			if node.Children[i].Depth != node.Children[j].Depth {
				return node.Children[i].Depth < node.Children[j].Depth
			}
			return node.Children[i].URL < node.Children[j].URL
		})
	}

	return tree
}

func hasCycle(node, targetParent *CrawlNode) bool {
	curr := targetParent
	visited := make(map[string]bool)
	for curr != nil {
		if curr.URL == node.URL {
			return true
		}
		if visited[curr.URL] {
			break
		}
		visited[curr.URL] = true
		if curr.ParentURL == "" {
			break
		}
		break
	}
	return false
}

// FlattenDFS traverses the tree depth-first, grouping children directly under parents.
func (t *CrawlTree) FlattenDFS() []*CrawlNode {
	if t.Root == nil {
		var list []*CrawlNode
		for _, n := range t.Nodes {
			list = append(list, n)
		}
		return list
	}

	var result []*CrawlNode
	visited := make(map[string]bool)

	var dfs func(n *CrawlNode)
	dfs = func(n *CrawlNode) {
		if n == nil || visited[n.URL] {
			return
		}
		visited[n.URL] = true
		result = append(result, n)

		for _, child := range n.Children {
			dfs(child)
		}
	}

	dfs(t.Root)

	// Append any disconnected nodes that were not reachable
	for _, n := range t.Nodes {
		if !visited[n.URL] {
			result = append(result, n)
		}
	}

	return result
}

// FlattenBFS traverses the tree level-by-level (by depth).
func (t *CrawlTree) FlattenBFS() []*CrawlNode {
	if t.Root == nil {
		return t.FlattenDFS()
	}

	var result []*CrawlNode
	visited := make(map[string]bool)
	queue := []*CrawlNode{t.Root}
	visited[t.Root.URL] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		result = append(result, curr)

		for _, child := range curr.Children {
			if !visited[child.URL] {
				visited[child.URL] = true
				queue = append(queue, child)
			}
		}
	}

	for _, n := range t.Nodes {
		if !visited[n.URL] {
			result = append(result, n)
		}
	}

	return result
}

// RenderASCIITree formats the crawl hierarchy as a clean Unicode/ASCII tree.
func (t *CrawlTree) RenderASCIITree() string {
	if t.Root == nil {
		return "(Empty Crawl Tree)"
	}

	var sb strings.Builder
	visited := make(map[string]bool)

	var renderNode func(n *CrawlNode, prefix string, isLast bool, isRoot bool)
	renderNode = func(n *CrawlNode, prefix string, isLast bool, isRoot bool) {
		if n == nil || visited[n.URL] {
			return
		}
		visited[n.URL] = true

		// Label builder
		label := n.Title
		if n.Product != nil && n.Product.Name != "" {
			label = n.Product.Name
		} else if label == "" {
			label = n.URL
		}
		if len(label) > 40 {
			label = label[:37] + "..."
		}

		infoBadge := fmt.Sprintf("[%s | d:%d]", n.PageType, n.Depth)
		if n.Product != nil && n.Product.Price > 0 {
			stock := "Out"
			if n.Product.InStock {
				stock = "InStock"
			}
			infoBadge = fmt.Sprintf("[%s | %.2f %s | %s | d:%d]", n.PageType, n.Product.Price, n.Product.Currency, stock, n.Depth)
		}

		if isRoot {
			sb.WriteString(fmt.Sprintf("%s %s %s\n", n.URL, label, infoBadge))
		} else {
			branch := "├── "
			if isLast {
				branch = "└── "
			}
			sb.WriteString(fmt.Sprintf("%s%s%s %s\n", prefix, branch, label, infoBadge))
		}

		childPrefix := prefix
		if !isRoot {
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
		}

		for i, child := range n.Children {
			renderNode(child, childPrefix, i == len(n.Children)-1, false)
		}
	}

	renderNode(t.Root, "", true, true)
	return sb.String()
}

// GetCategorySummary generates summary metrics for all categories in the tree.
func (t *CrawlTree) GetCategorySummary() []CategorySummary {
	var summaries []CategorySummary

	for cat, nodes := range t.ByCategory {
		var (
			prodCount int
			minPrice  = 0.0
			maxPrice  = 0.0
			sumPrice  = 0.0
			priceCount = 0
		)

		for _, n := range nodes {
			if n.Product != nil && (n.Product.Name != "" || n.Product.Price > 0) {
				prodCount++
				p := n.Product.Price
				if p > 0 {
					if minPrice == 0 || p < minPrice {
						minPrice = p
					}
					if p > maxPrice {
						maxPrice = p
					}
					sumPrice += p
					priceCount++
				}
			}
		}

		avgPrice := 0.0
		if priceCount > 0 {
			avgPrice = sumPrice / float64(priceCount)
		}

		summaries = append(summaries, CategorySummary{
			Name:         cat,
			TotalPages:   len(nodes),
			ProductCount: prodCount,
			MinPrice:     minPrice,
			MaxPrice:     maxPrice,
			AvgPrice:     avgPrice,
		})
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].TotalPages > summaries[j].TotalPages
	})

	return summaries
}

// GetProductSummary computes product metrics across the entire crawl.
func (t *CrawlTree) GetProductSummary() ProductSummary {
	var (
		total      int
		inStock    int
		outOfStock int
		minPrice   = 0.0
		maxPrice   = 0.0
		sumPrice   = 0.0
		priceCount = 0
		currMap    = make(map[string]bool)
	)

	for _, n := range t.Nodes {
		p := n.Product
		if p == nil || (p.Name == "" && p.Price == 0 && p.SKU == "") {
			continue
		}
		total++
		if p.InStock {
			inStock++
		} else {
			outOfStock++
		}

		if p.Price > 0 {
			if minPrice == 0 || p.Price < minPrice {
				minPrice = p.Price
			}
			if p.Price > maxPrice {
				maxPrice = p.Price
			}
			sumPrice += p.Price
			priceCount++
		}

		if p.Currency != "" {
			currMap[p.Currency] = true
		}
	}

	avgPrice := 0.0
	if priceCount > 0 {
		avgPrice = sumPrice / float64(priceCount)
	}

	var currencies []string
	for c := range currMap {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)

	return ProductSummary{
		TotalProducts:   total,
		InStockCount:    inStock,
		OutOfStockCount: outOfStock,
		MinPrice:        minPrice,
		MaxPrice:        maxPrice,
		AvgPrice:        avgPrice,
		Currencies:      currencies,
	}
}
