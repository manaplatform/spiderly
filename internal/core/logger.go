package core

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ─────────────────────────────────────────────
//  ANSI Color Codes
// ─────────────────────────────────────────────

const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"

	// Standard colors
	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	// Bright colors
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"

	// Background
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
)

// ─────────────────────────────────────────────
//  Phase Icons
// ─────────────────────────────────────────────

var phaseIcons = map[string]string{
	"init":      "🚀",
	"discovery": "🔍",
	"sitemap":   "🗺️ ",
	"crawling":  "🕸️ ",
	"complete":  "✨",
	"error":     "💥",
	"robots":    "🤖",
	"normalize": "🔗",
	"filter":    "🔬",
	"retry":     "🔄",
	"sink":      "💾",
	"shutdown":  "🛑",
}

// ─────────────────────────────────────────────
//  SummaryStats
// ─────────────────────────────────────────────

// SummaryStats holds aggregated crawl statistics for final display.
type SummaryStats struct {
	PagesScraped   int
	Errors         int
	Retries        int
	Skipped        int
	Duration       time.Duration
	PagesPerSecond float64
	TotalSize      int64
	StatusCodes    map[int]int
	RobotsBlocked  int
	DuplicatesSkip int
}

// ─────────────────────────────────────────────
//  Logger
// ─────────────────────────────────────────────

// Logger provides structured, colorized console output for the crawl pipeline.
// It is safe for concurrent use from multiple goroutines.
type Logger struct {
	noColor bool
	verbose bool

	mu         sync.Mutex
	pageCount  atomic.Int64
	errorCount atomic.Int64
	startTime  time.Time
}

// NewLogger creates a new Logger instance.
func NewLogger(noColor, verbose bool) *Logger {
	return &Logger{
		noColor:   noColor,
		verbose:   verbose,
		startTime: time.Now(),
	}
}

// ─────────────────────────────────────────────
//  Color Helpers
// ─────────────────────────────────────────────

// color wraps text with ANSI codes. Returns plain text when noColor is set.
func (l *Logger) color(c, text string) string {
	if l.noColor {
		return text
	}
	return c + text + Reset
}

// getStatusColor returns the ANSI color code for an HTTP status code.
func (l *Logger) getStatusColor(code int) string {
	switch {
	case code >= 200 && code < 300:
		return BrightGreen
	case code >= 300 && code < 400:
		return BrightYellow
	case code >= 400 && code < 500:
		return Yellow
	case code >= 500:
		return BrightRed
	default:
		return White
	}
}

// getPhaseIcon returns the emoji icon for a named phase.
func (l *Logger) getPhaseIcon(phase string) string {
	if icon, ok := phaseIcons[phase]; ok {
		return icon
	}
	return "📌"
}

// ─────────────────────────────────────────────
//  Header & Phase
// ─────────────────────────────────────────────

// Header prints the Spiderly banner.
func (l *Logger) Header() {
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Println()
	fmt.Println(l.color(BrightCyan+Bold, "  ╔═══════════════════════════════════════════════════════════════╗"))
	fmt.Println(l.color(BrightCyan+Bold, "  ║") + l.color(BrightMagenta+Bold, "     🕷️  SPIDERLY - High Performance Web Crawler              ") + l.color(BrightCyan+Bold, "║"))
	fmt.Println(l.color(BrightCyan+Bold, "  ╚═══════════════════════════════════════════════════════════════╝"))
	fmt.Println()
}

// Phase prints a phase separator with icon and message.
func (l *Logger) Phase(phase, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	icon := l.getPhaseIcon(phase)
	phaseText := l.color(BrightYellow+Bold, fmt.Sprintf(" %s %s ", icon, strings.ToUpper(phase)))
	fmt.Printf("\n%s %s\n", phaseText, l.color(White, message))
	fmt.Println(l.color(Dim, "  "+strings.Repeat("─", 60)))
}

// ─────────────────────────────────────────────
//  Log Levels
// ─────────────────────────────────────────────

// Info prints an informational message.
func (l *Logger) Info(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s %s\n", l.color(Blue, "ℹ"), l.color(White, msg))
}

// Success prints a success message.
func (l *Logger) Success(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s %s\n", l.color(BrightGreen, "✓"), l.color(Green, msg))
}

// Warning prints a warning message.
func (l *Logger) Warning(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s %s\n", l.color(Yellow, "⚠"), l.color(Yellow, msg))
}

// Error prints an error message.
func (l *Logger) Error(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s %s\n", l.color(BrightRed, "✗"), l.color(Red, msg))
}

// Verbose prints a debug-level message only when verbose mode is enabled.
func (l *Logger) Verbose(format string, args ...interface{}) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	fmt.Printf("  %s %s\n", l.color(Dim, "›"), l.color(Dim, msg))
}

// ─────────────────────────────────────────────
//  Page-Level Logging
// ─────────────────────────────────────────────

// PageScraped logs a successfully scraped page with status, timing, and title.
func (l *Logger) PageScraped(pageURL, title string, statusCode int, loadTimeMs int64) {
	count := l.pageCount.Add(1)

	displayURL := truncateString(pageURL, 50)
	displayTitle := truncateString(title, 35)
	if displayTitle == "" {
		displayTitle = "(no title)"
	}

	statusColor := l.getStatusColor(statusCode)
	statusStr := l.color(statusColor, fmt.Sprintf("[%d]", statusCode))

	countStr := l.color(Cyan, fmt.Sprintf("#%-4d", count))
	timeStr := l.color(Dim, fmt.Sprintf("%4dms", loadTimeMs))

	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Printf("  %s %s %s %s %s\n",
		countStr,
		statusStr,
		timeStr,
		l.color(BrightWhite, displayTitle),
		l.color(Dim, displayURL),
	)
}

// PageError logs a page-level error.
func (l *Logger) PageError(pageURL string, err error) {
	l.errorCount.Add(1)

	displayURL := truncateString(pageURL, 50)
	errMsg := truncateString(err.Error(), 40)

	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Printf("  %s %s %s\n",
		l.color(Red, "✗ ERR"),
		l.color(Dim, displayURL),
		l.color(Red, errMsg),
	)
}

// PageRetry logs a retry attempt for a page.
func (l *Logger) PageRetry(pageURL string, attempt, maxAttempts int, reason string) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	displayURL := truncateString(pageURL, 45)
	fmt.Printf("  %s %s %s %s\n",
		l.color(Yellow, "↻ RETRY"),
		l.color(Dim, fmt.Sprintf("[%d/%d]", attempt, maxAttempts)),
		l.color(Dim, displayURL),
		l.color(Yellow, truncateString(reason, 30)),
	)
}

// PageSkipped logs a skipped page (robots.txt, duplicate, etc.).
func (l *Logger) PageSkipped(pageURL, reason string) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	displayURL := truncateString(pageURL, 50)
	fmt.Printf("  %s %s %s\n",
		l.color(Dim, "⊘ SKIP"),
		l.color(Dim, displayURL),
		l.color(Dim, reason),
	)
}

// ─────────────────────────────────────────────
//  Link Discovery
// ─────────────────────────────────────────────

// LinkDiscovered logs a newly discovered link (verbose only).
func (l *Logger) LinkDiscovered(linkURL string, depth int) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	displayURL := truncateString(linkURL, 60)
	fmt.Printf("  %s %s %s\n",
		l.color(Dim, "  └─"),
		l.color(Dim, fmt.Sprintf("d%d", depth)),
		l.color(Dim, displayURL),
	)
}

// ─────────────────────────────────────────────
//  Sitemap Stats
// ─────────────────────────────────────────────

// SitemapStats prints a summary of sitemap discovery and filtering.
func (l *Logger) SitemapStats(total, filtered, sitemapCount int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Printf("\n  %s %s\n",
		l.color(Magenta, "📊"),
		l.color(BrightWhite+Bold, "Sitemap Analysis"),
	)
	fmt.Printf("     %s Sitemaps found:  %s\n", l.color(Dim, "├─"), l.color(Cyan, fmt.Sprintf("%d", sitemapCount)))
	fmt.Printf("     %s Total URLs:      %s\n", l.color(Dim, "├─"), l.color(Cyan, fmt.Sprintf("%d", total)))
	fmt.Printf("     %s After filtering: %s\n", l.color(Dim, "└─"), l.color(BrightGreen, fmt.Sprintf("%d", filtered)))
	fmt.Println()
}

// ─────────────────────────────────────────────
//  Robots.txt Logging
// ─────────────────────────────────────────────

// RobotsLoaded logs successful robots.txt parsing.
func (l *Logger) RobotsLoaded(targetURL string, ruleCount int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Printf("  %s %s %s\n",
		l.color(Green, "🤖"),
		l.color(White, "robots.txt loaded"),
		l.color(Dim, fmt.Sprintf("(%d rules for %s)", ruleCount, truncateString(targetURL, 40))),
	)
}

// RobotsBlocked logs a URL blocked by robots.txt.
func (l *Logger) RobotsBlocked(pageURL string) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	displayURL := truncateString(pageURL, 55)
	fmt.Printf("  %s %s %s\n",
		l.color(Yellow, "🤖 BLOCKED"),
		l.color(Dim, displayURL),
		l.color(Dim, "(robots.txt)"),
	)
}

// RobotsError logs a robots.txt fetch/parse error.
func (l *Logger) RobotsError(targetURL string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Printf("  %s %s %s\n",
		l.color(Yellow, "🤖 WARN"),
		l.color(Yellow, "robots.txt unavailable"),
		l.color(Dim, fmt.Sprintf("(%s: %s)", truncateString(targetURL, 30), truncateString(err.Error(), 30))),
	)
}

// ─────────────────────────────────────────────
//  Normalization Logging
// ─────────────────────────────────────────────

// DuplicateSkipped logs a URL skipped due to normalization dedup.
func (l *Logger) DuplicateSkipped(originalURL, normalizedURL string) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Printf("  %s %s\n",
		l.color(Dim, "⊘ DEDUP"),
		l.color(Dim, truncateString(normalizedURL, 60)),
	)
}

// ─────────────────────────────────────────────
//  Sink Logging
// ─────────────────────────────────────────────

// SinkFlushed logs a sink flush event (streaming output).
func (l *Logger) SinkFlushed(sinkName string, count int) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Printf("  %s %s %s\n",
		l.color(Dim, "💾 FLUSH"),
		l.color(Dim, sinkName),
		l.color(Dim, fmt.Sprintf("(%d pages)", count)),
	)
}

// SinkError logs a sink write error.
func (l *Logger) SinkError(sinkName string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Printf("  %s %s %s\n",
		l.color(Red, "💾 SINK ERR"),
		l.color(Red, sinkName),
		l.color(Red, truncateString(err.Error(), 40)),
	)
}

// ─────────────────────────────────────────────
//  Progress Bar
// ─────────────────────────────────────────────

// Progress renders an inline progress bar.
func (l *Logger) Progress(current, total int, phase string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if total == 0 {
		return
	}

	percent := float64(current) / float64(total) * 100
	barWidth := 30
	filled := int(float64(barWidth) * float64(current) / float64(total))
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	fmt.Printf("\r  %s %s %s %s",
		l.color(Cyan, fmt.Sprintf("%3.0f%%", percent)),
		l.color(BrightBlue, bar),
		l.color(White, fmt.Sprintf("%d/%d", current, total)),
		l.color(Dim, phase),
	)

	if current >= total {
		fmt.Println()
	}
}

// ─────────────────────────────────────────────
//  Summary
// ─────────────────────────────────────────────

// Summary prints the final crawl summary box.
func (l *Logger) Summary(stats SummaryStats) {
	l.mu.Lock()
	defer l.mu.Unlock()

	duration := stats.Duration.Round(time.Millisecond)

	fmt.Println()
	fmt.Println(l.color(BrightCyan, "  ╔═══════════════════════════════════════════════════════════════╗"))
	fmt.Println(l.color(BrightCyan, "  ║") + l.color(BrightGreen+Bold, "                    ✨ CRAWL COMPLETE ✨                       ") + l.color(BrightCyan, "║"))
	fmt.Println(l.color(BrightCyan, "  ╠═══════════════════════════════════════════════════════════════╣"))

	l.summaryRow("📄", "Pages Scraped", l.color(BrightGreen+Bold, fmt.Sprintf("%d", stats.PagesScraped)))
	l.summaryRow("❌", "Errors", l.color(Yellow, fmt.Sprintf("%d", stats.Errors)))

	if stats.Retries > 0 {
		l.summaryRow("🔄", "Retries", l.color(Yellow, fmt.Sprintf("%d", stats.Retries)))
	}
	if stats.RobotsBlocked > 0 {
		l.summaryRow("🤖", "Robots Blocked", l.color(Dim, fmt.Sprintf("%d", stats.RobotsBlocked)))
	}
	if stats.DuplicatesSkip > 0 {
		l.summaryRow("⊘ ", "Duplicates Skipped", l.color(Dim, fmt.Sprintf("%d", stats.DuplicatesSkip)))
	}
	if stats.Skipped > 0 {
		l.summaryRow("⏭️ ", "Skipped", l.color(Dim, fmt.Sprintf("%d", stats.Skipped)))
	}

	l.summaryRow("⏱️ ", "Duration", l.color(Cyan, duration.String()))
	l.summaryRow("⚡", "Speed", l.color(Cyan, fmt.Sprintf("%.1f pages/sec", stats.PagesPerSecond)))

	if stats.TotalSize > 0 {
		l.summaryRow("📦", "Total Size", l.color(Cyan, humanizeBytes(stats.TotalSize)))
	}

	fmt.Println(l.color(BrightCyan, "  ╠═══════════════════════════════════════════════════════════════╣"))

	// HTTP Status breakdown
	fmt.Println(l.color(BrightCyan, "  ║") + l.color(White+Bold, "  HTTP Status Breakdown:                                      ") + l.color(BrightCyan, "║"))
	for code, count := range stats.StatusCodes {
		emoji := statusEmoji(code)
		clr := l.getStatusColor(code)
		fmt.Printf(l.color(BrightCyan, "  ║")+"     %s %s: %-46s"+l.color(BrightCyan, "║")+"\n",
			emoji,
			l.color(clr, fmt.Sprintf("%d", code)),
			l.color(clr, fmt.Sprintf("%d", count)),
		)
	}

	fmt.Println(l.color(BrightCyan, "  ╚═══════════════════════════════════════════════════════════════╝"))
	fmt.Println()
}

// summaryRow prints a single row inside the summary box.
// Must be called with l.mu held.
func (l *Logger) summaryRow(emoji, label, value string) {
	fmt.Printf(l.color(BrightCyan, "  ║")+"  %s %-18s %-40s"+l.color(BrightCyan, "║")+"\n",
		emoji, label+":", value,
	)
}

// ─────────────────────────────────────────────
//  Counters (thread-safe reads)
// ─────────────────────────────────────────────

// PageCount returns the current number of logged scraped pages.
func (l *Logger) PageCount() int64 {
	return l.pageCount.Load()
}

// ErrorCount returns the current number of logged errors.
func (l *Logger) ErrorCount() int64 {
	return l.errorCount.Load()
}