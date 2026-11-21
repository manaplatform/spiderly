// internal/extractor/jsonld_test.go
package extractor

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestExtractJSONLD_ProductSingleOffer(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
<script type="application/ld+json">
{
  "@context": "https://schema.org/",
  "@type": "Product",
  "name": "Wireless Noise Cancelling Headphones",
  "image": [
    "https://example.com/photos/1x1/photo.jpg",
    "https://example.com/photos/4x3/photo.jpg"
  ],
  "description": "High fidelity audio with adaptive noise cancellation.",
  "sku": "WH-1000XM5",
  "mpn": "925872",
  "gtin13": "1234567890123",
  "brand": {
    "@type": "Brand",
    "name": "Sony"
  },
  "offers": {
    "@type": "Offer",
    "url": "https://example.com/headphones",
    "priceCurrency": "USD",
    "price": "399.99",
    "priceValidUntil": "2026-12-31",
    "itemCondition": "https://schema.org/NewCondition",
    "availability": "https://schema.org/InStock"
  },
  "aggregateRating": {
    "@type": "AggregateRating",
    "ratingValue": 4.8,
    "reviewCount": 1250
  }
}
</script>
</head>
<body></body>
</html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("failed to parse HTML: %v", err)
	}

	sd := ExtractJSONLD(doc.Selection)
	if sd == nil {
		t.Fatalf("expected structured data, got nil")
	}

	if sd.Name != "Wireless Noise Cancelling Headphones" {
		t.Errorf("expected name 'Wireless Noise Cancelling Headphones', got %q", sd.Name)
	}
	if sd.Brand != "Sony" {
		t.Errorf("expected brand 'Sony', got %q", sd.Brand)
	}
	if sd.SKU != "WH-1000XM5" {
		t.Errorf("expected SKU 'WH-1000XM5', got %q", sd.SKU)
	}
	if sd.GTIN != "1234567890123" {
		t.Errorf("expected GTIN '1234567890123', got %q", sd.GTIN)
	}
	if sd.MPN != "925872" {
		t.Errorf("expected MPN '925872', got %q", sd.MPN)
	}
	if sd.Price != 399.99 {
		t.Errorf("expected price 399.99, got %f", sd.Price)
	}
	if sd.Currency != "USD" {
		t.Errorf("expected currency 'USD', got %q", sd.Currency)
	}
	if sd.Availability != "InStock" || !sd.InStock {
		t.Errorf("expected InStock=true, got %s / %v", sd.Availability, sd.InStock)
	}
	if sd.Rating != 4.8 {
		t.Errorf("expected rating 4.8, got %f", sd.Rating)
	}
	if sd.ReviewCount != 1250 {
		t.Errorf("expected review count 1250, got %d", sd.ReviewCount)
	}
	if len(sd.Images) != 2 {
		t.Errorf("expected 2 images, got %d", len(sd.Images))
	}
}

func TestExtractJSONLD_GraphWithBreadcrumbsAndAggregateOffer(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "BreadcrumbList",
      "itemListElement": [
        {
          "@type": "ListItem",
          "position": 1,
          "name": "Home",
          "item": "https://example.com"
        },
        {
          "@type": "ListItem",
          "position": 2,
          "name": "Footwear",
          "item": "https://example.com/footwear"
        },
        {
          "@type": "ListItem",
          "position": 3,
          "name": "Running Shoes",
          "item": "https://example.com/footwear/running"
        }
      ]
    },
    {
      "@type": ["Product", "IndividualProduct"],
      "name": "Pro Speed Runner",
      "brand": "Nike",
      "offers": {
        "@type": "AggregateOffer",
        "lowPrice": 129.95,
        "highPrice": 159.95,
        "priceCurrency": "EUR",
        "availability": "https://schema.org/OutOfStock"
      }
    }
  ]
}
</script>
</head>
<body></body>
</html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("failed to parse HTML: %v", err)
	}

	sd := ExtractJSONLD(doc.Selection)
	if sd == nil {
		t.Fatalf("expected structured data, got nil")
	}

	if sd.Name != "Pro Speed Runner" {
		t.Errorf("expected name 'Pro Speed Runner', got %q", sd.Name)
	}
	if sd.Brand != "Nike" {
		t.Errorf("expected brand 'Nike', got %q", sd.Brand)
	}
	if sd.Price != 129.95 {
		t.Errorf("expected lowPrice 129.95, got %f", sd.Price)
	}
	if sd.Currency != "EUR" {
		t.Errorf("expected currency 'EUR', got %q", sd.Currency)
	}
	if sd.InStock {
		t.Errorf("expected InStock=false for OutOfStock, got true")
	}
	if len(sd.Breadcrumbs) != 3 {
		t.Fatalf("expected 3 breadcrumbs, got %d (%v)", len(sd.Breadcrumbs), sd.Breadcrumbs)
	}
	if sd.Breadcrumbs[0] != "Home" || sd.Breadcrumbs[1] != "Footwear" || sd.Breadcrumbs[2] != "Running Shoes" {
		t.Errorf("unexpected breadcrumb order: %v", sd.Breadcrumbs)
	}
}

func TestExtractJSONLD_DOMBreadcrumbFallback(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<nav aria-label="breadcrumb">
  <ol class="breadcrumb">
    <li class="breadcrumb-item"><a href="/">Home</a></li>
    <li class="breadcrumb-item"><a href="/shop">Shop</a></li>
    <li class="breadcrumb-item active" aria-current="page">Apparel</li>
  </ol>
</nav>
</body>
</html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("failed to parse HTML: %v", err)
	}

	crumbs := ExtractDOMBreadcrumbs(doc.Selection)
	if len(crumbs) != 3 {
		t.Fatalf("expected 3 DOM breadcrumbs, got %d (%v)", len(crumbs), crumbs)
	}
	if crumbs[0] != "Home" || crumbs[1] != "Shop" || crumbs[2] != "Apparel" {
		t.Errorf("unexpected breadcrumb items: %v", crumbs)
	}
}
