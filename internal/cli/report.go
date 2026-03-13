package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"spiderly/internal/output"
	"spiderly/internal/report"

	"github.com/spf13/cobra"
)

func newReportCmd() *cobra.Command {
	var input string
	var format string
	var out string
	var title string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a report from existing crawl JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(input) == "" {
				return errors.New("missing --input")
			}
			if strings.TrimSpace(format) == "" {
				return errors.New("missing --format")
			}

			data, err := os.ReadFile(input)
			if err != nil {
				return fmt.Errorf("read input: %w", err)
			}

			var result report.CrawlReport
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("decode input JSON: %w", err)
			}

			switch strings.ToLower(format) {
			case "markdown", "md":
				content, err := output.RenderMarkdown(result, title)
				if err != nil {
					return err
				}
				if out == "" {
					fmt.Print(content)
					return nil
				}
				return os.WriteFile(out, []byte(content), 0o644)

			case "json":
				content, err := output.RenderJSON(result, true)
				if err != nil {
					return err
				}
				if out == "" {
					fmt.Print(content)
					return nil
				}
				return os.WriteFile(out, []byte(content), 0o644)

			default:
				return fmt.Errorf("unsupported format: %s", format)
			}
		},
	}

	cmd.Flags().StringVar(&input, "input", "", "Input crawl JSON file")
	cmd.Flags().StringVar(&format, "format", "markdown", "Output format: markdown|json")
	cmd.Flags().StringVar(&out, "out", "", "Output file path, defaults to stdout")
	cmd.Flags().StringVar(&title, "title", "Spiderly Crawl Report", "Report title")

	return cmd
}
