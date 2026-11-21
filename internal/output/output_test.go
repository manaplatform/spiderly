// internal/output/output_test.go
package output

import (
	"strings"
	"testing"

	"spiderly/internal/engine"
	"spiderly/internal/models"
	"spiderly/internal/report"
)

func TestRenderMarkdown_FullHierarchy(t *testing.T) {
	rep := report.CrawlReport{
		Meta: report.Meta{
			StartURL:  "https://shop.example.com",
			CrawlTime: "2026-11-22 02:00:00",
			Duration:  "1.5s",
		},
		Statistics: report.Statistics{
			TotalPages:    3,
			TotalProducts: 1,
			InStockCount:  1,
			MaxDepth:      2,
		},
		TreeASCII: "https://shop.example.com [root]\n└── /shoes [category]\n    └── /shoes/runner [product]",
		Categories: []engine.CategorySummary{
			{
				Name:         "Shoes",
				TotalPages:   2,
				ProductCount: 1,
				MinPrice:     100.0,
				MaxPrice:     100.0,
				AvgPrice:     100.0,
			},
		},
		Pages: []report.Page{
			{
				URL:        "https://shop.example.com",
				Title:      "Shop Home",
				StatusCode: 200,
				Depth:      0,
			},
			{
				URL:        "https://shop.example.com/shoes",
				Title:      "Shoes",
				StatusCode: 200,
				Depth:      1,
				ParentURL:  "https://shop.example.com",
				Category:   "Shoes",
			},
			{
				URL:        "https://shop.example.com/shoes/runner",
				Title:      "Pro Runner",
				StatusCode: 200,
				Depth:      2,
				ParentURL:  "https://shop.example.com/shoes",
				Category:   "Shoes",
				Product: &models.ProductInfo{
					Name:         "Pro Runner",
					Price:        100.0,
					Currency:     "USD",
					Availability: "InStock",
					InStock:      true,
					Rating:       4.8,
					ReviewCount:  50,
					SKU:          "RUN-100",
				},
			},
		},
	}

	md, err := RenderMarkdown(rep, "Shop Crawl Report")
	if err != nil {
		t.Fatalf("RenderMarkdown failed: %v", err)
	}

	if !strings.Contains(md, "# Shop Crawl Report") {
		t.Errorf("missing title in markdown")
	}
	if !strings.Contains(md, "Site Hierarchy Tree") {
		t.Errorf("missing hierarchy tree section")
	}
	if !strings.Contains(md, "Category & Section Breakdown") {
		t.Errorf("missing category breakdown")
	}
	if !strings.Contains(md, "Structured Product Catalog") {
		t.Errorf("missing product catalog table")
	}
	if !strings.Contains(md, "Pro Runner") || !strings.Contains(md, "100.00 USD") {
		t.Errorf("missing product details in table")
	}
}

func TestRenderJSON_FullHierarchy(t *testing.T) {
	rep := report.CrawlReport{
		Meta: report.Meta{StartURL: "https://shop.example.com"},
		Statistics: report.Statistics{
			TotalPages: 1,
		},
		Pages: []report.Page{
			{
				URL:        "https://shop.example.com",
				Title:      "Shop",
				StatusCode: 200,
			},
		},
	}

	jsonStr, err := RenderJSON(rep, true)
	if err != nil {
		t.Fatalf("RenderJSON failed: %v", err)
	}
	if !strings.Contains(jsonStr, `"start_url": "https://shop.example.com"`) {
		t.Errorf("missing start_url in JSON: %s", jsonStr)
	}
}
