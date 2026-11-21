package report

import (
	"spiderly/internal/engine"
	"spiderly/internal/models"
)

type CrawlReport struct {
	Meta          Meta                     `json:"meta"`
	Statistics    Statistics               `json:"statistics"`
	Categories    []engine.CategorySummary `json:"categories,omitempty"`
	ProductStats  *engine.ProductSummary   `json:"product_stats,omitempty"`
	HierarchyTree *engine.CrawlNode        `json:"hierarchy_tree,omitempty"`
	TreeASCII     string                   `json:"tree_ascii,omitempty"`
	Pages         []Page                   `json:"pages"`
}

type Meta struct {
	StartURL  string `json:"start_url"`
	CrawlTime string `json:"crawl_time,omitempty"`
	Duration  string `json:"duration,omitempty"`
}

type Statistics struct {
	TotalPages    int `json:"total_pages"`
	TotalProducts int `json:"total_products,omitempty"`
	InStockCount  int `json:"in_stock_count,omitempty"`
	MaxDepth      int `json:"max_depth,omitempty"`
}

type Page struct {
	URL            string                 `json:"url"`
	Title          string                 `json:"title,omitempty"`
	StatusCode     int                    `json:"status_code"`
	Depth          int                    `json:"depth"`
	ParentURL      string                 `json:"parent_url,omitempty"`
	Category       string                 `json:"category,omitempty"`
	Breadcrumbs    []string               `json:"breadcrumbs,omitempty"`
	PageType       string                 `json:"page_type,omitempty"`
	Product        *models.ProductInfo    `json:"product,omitempty"`
	StructuredData *models.StructuredData `json:"structured_data,omitempty"`
	Links          []Link                 `json:"links,omitempty"`
}

type Link struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
}
