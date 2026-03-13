package output

import (
	"fmt"

	"spiderly/internal/report"
)

func PrintSummary(r report.CrawlReport) {
	fmt.Println("Crawl summary")
	fmt.Println("-------------")
	fmt.Printf("Start URL:   %s\n", r.Meta.StartURL)
	fmt.Printf("Pages:       %d\n", len(r.Pages))
}
