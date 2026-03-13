package output

import (
	"fmt"
	"os"
	"strings"

	"spiderly/internal/report"
)

func RenderMarkdown(r report.CrawlReport, title string) (string, error) {
	var b strings.Builder

	if title == "" {
		title = "Spiderly Crawl Report"
	}

	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "- Start URL: %s\n", r.Meta.StartURL)
	fmt.Fprintf(&b, "- Total pages: %d\n\n", len(r.Pages))

	if len(r.Pages) > 0 {
		b.WriteString("## Pages\n\n")
		for _, p := range r.Pages {
			fmt.Fprintf(&b, "- `%s` (%d)\n", p.URL, p.StatusCode)
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func WriteMarkdownFile(path string, r report.CrawlReport, title string) error {
	s, err := RenderMarkdown(r, title)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(s), 0o644)
}
