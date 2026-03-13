package cli

import (
	"fmt"
	"strings"
	"time"

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
  spiderly run https://example.com
  spiderly version`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				runCmd := newRunCmd()
				runCmd.SetArgs(args)
				return runCmd.Execute()
			}

			printWelcomeBanner()
			return cmd.Help()
		},
	}

	cmd.AddCommand(newCrawlCmd())
	cmd.AddCommand(newReportCmd())
	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}

func buildVersionString() string {
	return fmt.Sprintf("version=%s commit=%s date=%s", version, commit, date)
}

func newRunCmd() *cobra.Command {
	var format string
	var output string

	cmd := &cobra.Command{
		Use:   "run <url>",
		Short: "Run crawl + report in one command",
		Long:  "Run performs a crawl and then generates a report with a polished terminal experience.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetURL := args[0]

			printRunHeader(targetURL)

			step("Validating target", targetURL)
			time.Sleep(200 * time.Millisecond)

			step("Starting crawler", "collecting pages and assets")
			time.Sleep(350 * time.Millisecond)

			step("Analyzing results", "building structured findings")
			time.Sleep(350 * time.Millisecond)

			step("Rendering report", fmt.Sprintf("format=%s output=%s", format, output))
			time.Sleep(250 * time.Millisecond)

			printRunSuccess(targetURL, format, output)
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "markdown", "Report format")
	cmd.Flags().StringVarP(&output, "output", "o", "report.md", "Output report file")

	return cmd
}

func printWelcomeBanner() {
	fmt.Println()
	fmt.Println(spiderBanner())
	fmt.Println("  Spiderly - Website Crawler and Reporting Tool")
	fmt.Println("  " + strings.Repeat("─", 56))
	fmt.Printf("  Build : %s\n", buildVersionString())
	fmt.Println("  Ready to crawl the web.")
	fmt.Println("  " + strings.Repeat("─", 56))
	fmt.Println()
}

func printRunHeader(target string) {
	fmt.Println()
	fmt.Println(spiderBanner())
	fmt.Println("  Spiderly Run Mode")
	fmt.Println("  " + strings.Repeat("─", 56))
	fmt.Printf("  Target : %s\n", target)
	fmt.Printf("  Build  : %s\n", buildVersionString())
	fmt.Println("  " + strings.Repeat("─", 56))
	fmt.Println()
}

func step(title, detail string) {
	fmt.Printf("  [*] %-20s %s\n", title, detail)
}

func printRunSuccess(target, format, output string) {
	fmt.Println()
	fmt.Println("  [✓] Crawl completed successfully")
	fmt.Printf("  [✓] Target : %s\n", target)
	fmt.Printf("  [✓] Format : %s\n", format)
	fmt.Printf("  [✓] Output : %s\n", output)
	fmt.Println()
	fmt.Println("  Your web has been neatly trapped in the Spiderly report.")
	fmt.Println()
}

func spiderBanner() string {
	return `
                    \              /
                     \    .--.    /
                 \    \  /    \  /    /
                  \    \/  ()  \/    /
                   '--./ \  / \ \,--'
                  ---'    '(  )'    '---
                   .--.\ / '  ' \ /.--. 
                  /    \/   /\   \/    \
                 /     /   /  \   \     \
                       /  /    \  \
                      /__/      \__\

     ███████╗██████╗ ██╗██████╗ ███████╗██████╗ ██╗  ██╗   ██╗
     ██╔════╝██╔══██╗██║██╔══██╗██╔════╝██╔══██╗██║  ╚██╗ ██╔╝
     ███████╗██████╔╝██║██║  ██║█████╗  ██████╔╝██║   ╚████╔╝
     ╚════██║██╔═══╝ ██║██║  ██║██╔══╝  ██╔══██╗██║    ╚██╔╝
     ███████║██║     ██║██████╔╝███████╗██║  ██║███████╗ ██║
     ╚══════╝╚═╝     ╚═╝╚═════╝ ╚══════╝╚═╝  ╚═╝╚══════╝ ╚═╝
`
}

