// internal/engine/tree_test.go
package engine

import (
	"strings"
	"testing"
	"time"

	"spiderly/internal/models"
)

func TestBuildTree_HierarchyAndCategorization(t *testing.T) {
	pages := []models.ScrapedPage{
		{
			URL:        "https://example.com",
			Title:      "Example Home",
			StatusCode: 200,
			Depth:      0,
		},
		{
			URL:        "https://example.com/shop",
			ParentURL:  "https://example.com",
			Title:      "Shop Section",
			StatusCode: 200,
			Depth:      1,
		},
		{
			URL:        "https://example.com/shop/shoes",
			ParentURL:  "https://example.com/shop",
			Title:      "Shoes Category",
			StatusCode: 200,
			Depth:      2,
		},
		{
			URL:        "https://example.com/shop/shoes/runner-1",
			ParentURL:  "https://example.com/shop/shoes",
			Title:      "Runner 1",
			StatusCode: 200,
			Depth:      3,
			Product: &models.ProductInfo{
				Name:         "Fast Runner",
				Price:        120.00,
				Currency:     "USD",
				Availability: "InStock",
				InStock:      true,
				Rating:       4.5,
			},
		},
		{
			URL:        "https://example.com/shop/shoes/runner-2",
			ParentURL:  "https://example.com/shop/shoes",
			Title:      "Runner 2",
			StatusCode: 200,
			Depth:      3,
			Product: &models.ProductInfo{
				Name:         "Trail Runner",
				Price:        150.00,
				Currency:     "USD",
				Availability: "OutOfStock",
				InStock:      false,
				Rating:       4.8,
			},
		},
		{
			URL:        "https://example.com/about",
			ParentURL:  "https://example.com",
			Title:      "About Us",
			StatusCode: 200,
			Depth:      1,
		},
	}

	tree := BuildTree(pages, "https://example.com")
	if tree == nil {
		t.Fatalf("expected non-nil tree")
	}
	if tree.Root == nil {
		t.Fatalf("expected root node")
	}
	if tree.Root.URL != "https://example.com" {
		t.Errorf("expected root URL 'https://example.com', got %s", tree.Root.URL)
	}

	// Verify root has 2 children (/shop and /about)
	if len(tree.Root.Children) != 2 {
		t.Errorf("expected root to have 2 children, got %d", len(tree.Root.Children))
	}

	// Verify /shop has child /shop/shoes
	var shopNode *CrawlNode
	for _, child := range tree.Root.Children {
		if strings.Contains(child.URL, "/shop") && !strings.Contains(child.URL, "/about") {
			shopNode = child
			break
		}
	}
	if shopNode == nil {
		t.Fatalf("shop node not found under root")
	}
	if len(shopNode.Children) != 1 {
		t.Errorf("expected shop to have 1 child (/shop/shoes), got %d", len(shopNode.Children))
	}

	// Verify shoes has 2 product children
	shoesNode := shopNode.Children[0]
	if len(shoesNode.Children) != 2 {
		t.Errorf("expected shoes node to have 2 children, got %d", len(shoesNode.Children))
	}

	// Test DFS flattening
	dfs := tree.FlattenDFS()
	if len(dfs) != len(pages) {
		t.Errorf("expected DFS to return %d nodes, got %d", len(pages), len(dfs))
	}

	// Test Category Summary
	catSummary := tree.GetCategorySummary()
	if len(catSummary) == 0 {
		t.Errorf("expected category summaries, got none")
	}

	// Test Product Summary
	prodSummary := tree.GetProductSummary()
	if prodSummary.TotalProducts != 2 {
		t.Errorf("expected 2 products, got %d", prodSummary.TotalProducts)
	}
	if prodSummary.InStockCount != 1 {
		t.Errorf("expected 1 in stock, got %d", prodSummary.InStockCount)
	}
	if prodSummary.OutOfStockCount != 1 {
		t.Errorf("expected 1 out of stock, got %d", prodSummary.OutOfStockCount)
	}
	if prodSummary.MinPrice != 120.00 || prodSummary.MaxPrice != 150.00 {
		t.Errorf("expected min/max 120/150, got %f/%f", prodSummary.MinPrice, prodSummary.MaxPrice)
	}

	// Test ASCII tree rendering
	ascii := tree.RenderASCIITree()
	if !strings.Contains(ascii, "Example Home") || !strings.Contains(ascii, "Fast Runner") {
		t.Errorf("expected ASCII tree to contain page titles, got:\n%s", ascii)
	}
}

func TestFilterAndSort(t *testing.T) {
	now := time.Now()
	nodes := []*CrawlNode{
		{
			URL:      "https://example.com/p1",
			Title:    "Alpha Budget Shoe",
			Category: "Shoes",
			Depth:    2,
			Product: &models.ProductInfo{
				Name:    "Alpha Budget Shoe",
				Price:   50.0,
				InStock: true,
				Rating:  3.5,
			},
			Page: &models.ScrapedPage{ScrapedAt: now.Add(-2 * time.Hour)},
		},
		{
			URL:      "https://example.com/p2",
			Title:    "Beta Premium Shoe",
			Category: "Shoes",
			Depth:    2,
			Product: &models.ProductInfo{
				Name:    "Beta Premium Shoe",
				Price:   200.0,
				InStock: false,
				Rating:  4.9,
			},
			Page: &models.ScrapedPage{ScrapedAt: now.Add(-1 * time.Hour)},
		},
		{
			URL:      "https://example.com/p3",
			Title:    "Gamma Luxury Jacket",
			Category: "Apparel",
			Depth:    1,
			Product: &models.ProductInfo{
				Name:    "Gamma Luxury Jacket",
				Price:   300.0,
				InStock: true,
				Rating:  4.8,
			},
			Page: &models.ScrapedPage{ScrapedAt: now},
		},
	}

	// Filter in-stock only
	inStockNodes := FilterNodes(nodes, FilterOptions{InStockOnly: true})
	if len(inStockNodes) != 2 {
		t.Errorf("expected 2 in-stock nodes, got %d", len(inStockNodes))
	}

	// Filter by category
	apparelNodes := FilterNodes(nodes, FilterOptions{Category: "apparel"})
	if len(apparelNodes) != 1 || apparelNodes[0].Title != "Gamma Luxury Jacket" {
		t.Errorf("expected 1 apparel node, got %v", apparelNodes)
	}

	// Filter by price range
	minPrice := 100.0
	maxPrice := 250.0
	priceFiltered := FilterNodes(nodes, FilterOptions{MinPrice: &minPrice, MaxPrice: &maxPrice})
	if len(priceFiltered) != 1 || priceFiltered[0].Title != "Beta Premium Shoe" {
		t.Errorf("expected 1 node between 100 and 250, got %v", priceFiltered)
	}

	// Sort by price ascending
	sortedByPrice := SortNodes(nodes, SortPriceAsc, nil)
	if sortedByPrice[0].Product.Price != 50.0 || sortedByPrice[2].Product.Price != 300.0 {
		t.Errorf("price asc sort failed: 1st=%.2f, 3rd=%.2f", sortedByPrice[0].Product.Price, sortedByPrice[2].Product.Price)
	}

	// Sort by rating descending
	sortedByRating := SortNodes(nodes, SortRating, nil)
	if sortedByRating[0].Product.Rating != 4.9 {
		t.Errorf("rating desc sort failed: 1st=%.2f", sortedByRating[0].Product.Rating)
	}
}
