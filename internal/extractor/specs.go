// internal/extractor/specs.go
package extractor

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// specTableSelectors covers common spec/attribute table patterns.
var specTableSelectors = []string{
	"table.specifications tr",
	"table.specs tr",
	"table.product-specs tr",
	".product-specs tr",
	".specifications-table tr",
	".product-attributes table tr",
	".params-list tr",
	// Persian e-commerce common patterns
	"table.product-params tr",
	".product-parameters tr",
	".content-expert-table tr",
	".data-sheet tr",
}

// specDLSelectors covers definition-list based spec layouts.
var specDLSelectors = []string{
	"dl.specifications",
	"dl.specs",
	"dl.product-specs",
	".product-attributes dl",
	".product-specs dl",
	".specifications dl",
	// Persian e-commerce
	".params-list dl",
	".product-parameters dl",
}

// specKVSelectors covers key-value list patterns (li > span+span).
var specKVSelectors = []string{
	"ul.specifications li",
	"ul.specs li",
	".product-specs li",
	".product-attributes li",
	".specifications-list li",
	".params-list li",
}

// extractSpecifications extracts product specifications from tables,
// definition lists, and key-value list patterns.
func extractSpecifications(doc *goquery.Selection) map[string]string {
	specs := make(map[string]string)

	// 1. Table rows: th/first-td as key, last-td as value
	extractSpecsFromTables(doc, specs)

	// 2. Definition lists: dt as key, dd as value
	extractSpecsFromDL(doc, specs)

	// 3. Key-value lists: li with two child spans or strong+span
	extractSpecsFromKVList(doc, specs)

	return specs
}

// extractSpecsFromTables pulls specs from <table> rows.
func extractSpecsFromTables(doc *goquery.Selection, specs map[string]string) {
	combined := strings.Join(specTableSelectors, ", ")
	doc.Find(combined).Each(func(_ int, row *goquery.Selection) {
		key := cleanSpecText(row.Find("th, td:first-child").First().Text())
		value := cleanSpecText(row.Find("td:last-child").Text())
		if key != "" && value != "" && key != value {
			if _, exists := specs[key]; !exists {
				specs[key] = value
			}
		}
	})
}

// extractSpecsFromDL pulls specs from <dl> definition lists.
func extractSpecsFromDL(doc *goquery.Selection, specs map[string]string) {
	combined := strings.Join(specDLSelectors, ", ")
	doc.Find(combined).Each(func(_ int, dl *goquery.Selection) {
		dl.Find("dt").Each(func(_ int, dt *goquery.Selection) {
			key := cleanSpecText(dt.Text())
			// Next sibling should be <dd>
			dd := dt.Next()
			if dd.Length() == 0 || goquery.NodeName(dd) != "dd" {
				return
			}
			value := cleanSpecText(dd.Text())
			if key != "" && value != "" {
				if _, exists := specs[key]; !exists {
					specs[key] = value
				}
			}
		})
	})
}

// extractSpecsFromKVList pulls specs from <ul>/<li> with paired children.
func extractSpecsFromKVList(doc *goquery.Selection, specs map[string]string) {
	combined := strings.Join(specKVSelectors, ", ")
	doc.Find(combined).Each(func(_ int, li *goquery.Selection) {
		children := li.Children()
		if children.Length() < 2 {
			return
		}
		key := cleanSpecText(children.First().Text())
		value := cleanSpecText(children.Last().Text())
		if key != "" && value != "" && key != value {
			if _, exists := specs[key]; !exists {
				specs[key] = value
			}
		}
	})
}

// cleanSpecText trims whitespace, colons, and normalises internal spaces.
func cleanSpecText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ":")
	s = strings.TrimRight(s, "：") // fullwidth colon
	// collapse internal whitespace
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
