package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func SetBuildInfo(v, c, d string) {
	version = v
	commit = c
	date = d
}

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spiderly",
		Short: "Spiderly is a website crawler and reporting tool",
		Long: `Spiderly crawls websites and produces structured reports.

Examples:
  spiderly crawl https://example.com
  spiderly report --input crawl.json --format markdown
  spiderly version`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(newCrawlCmd())
	cmd.AddCommand(newReportCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func buildVersionString() string {
	return fmt.Sprintf("version=%s commit=%s date=%s", version, commit, date)
}
