// internal/chunker/logger.go
package chunker

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ─────────────────────────────────────────────
//  Chunker Logger
// ─────────────────────────────────────────────

// ChunkerLogger handles beautiful console output for chunker
type ChunkerLogger struct {
	noColor bool
	verbose bool
	mu      sync.Mutex
}

// NewChunkerLogger creates a new logger
func NewChunkerLogger(noColor, verbose bool) *ChunkerLogger {
	return &ChunkerLogger{
		noColor: noColor,
		verbose: verbose,
	}
}

func (l *ChunkerLogger) color(c, text string) string {
	if l.noColor {
		return text
	}
	return c + text + Reset
}

// ─────────────────────────────────────────────
//  Header & Info
// ─────────────────────────────────────────────

func (l *ChunkerLogger) Header() {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	fmt.Println()
	fmt.Println(l.color(BrightCyan+Bold, "  ╔═══════════════════════════════════════════════════════════════════════╗"))
	fmt.Println(l.color(BrightCyan+Bold, "  ║") + l.color(BrightMagenta+Bold, "     🕷️  SPIDERLY CHUNKER - Parallel Multi-Process Crawler              ") + l.color(BrightCyan+Bold, "║"))
	fmt.Println(l.color(BrightCyan+Bold, "  ╚═══════════════════════════════════════════════════════════════════════╝"))
	fmt.Println()
}

func (l *ChunkerLogger) ChunkingInfo(chunks, totalURLs, workers, chunkSize int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	fmt.Println(l.color(BrightYellow+Bold, "  ⚡ CHUNKING CONFIGURATION"))
	fmt.Println(l.color(Dim, "  "+strings.Repeat("─", 70)))
	fmt.Println()
	
	// Visual chunk representation
	fmt.Printf("  %s %s\n", 
		l.color(Cyan, "📦 Total URLs:"),
		l.color(BrightWhite+Bold, fmt.Sprintf("%d", totalURLs)))
	fmt.Printf("  %s %s\n",
		l.color(Cyan, "🧩 Chunks:"),
		l.color(BrightWhite+Bold, fmt.Sprintf("%d chunks × %d URLs each", chunks, chunkSize)))
	fmt.Printf("  %s %s\n",
		l.color(Cyan, "👷 Workers:"),
		l.color(BrightWhite+Bold, fmt.Sprintf("%d parallel processes", workers)))
	
	fmt.Println()
	
	// Visual worker allocation
	fmt.Printf("  %s ", l.color(White, "Worker Pool:"))
	for i := 0; i < workers; i++ {
		colors := []string{BrightGreen, BrightYellow, BrightBlue, BrightMagenta, BrightCyan}
		c := colors[i%len(colors)]
		fmt.Printf("%s ", l.color(c, fmt.Sprintf("W%d", i+1)))
	}
	fmt.Println()
	
	// Chunk visualization
	fmt.Printf("  %s ", l.color(White, "Chunks:     "))
	displayed := chunks
	if displayed > 20 {
		displayed = 20
	}
	for i := 0; i < displayed; i++ {
		fmt.Printf("%s", l.color(Dim, "▪"))
	}
	if chunks > 20 {
		fmt.Printf("%s", l.color(Dim, fmt.Sprintf(" +%d more", chunks-20)))
	}
	fmt.Println()
	
	fmt.Println()
	fmt.Println(l.color(BrightYellow+Bold, "  🚀 STARTING PARALLEL CRAWL"))
	fmt.Println(l.color(Dim, "  "+strings.Repeat("─", 70)))
	fmt.Println()
}

// ─────────────────────────────────────────────
//  Chunk Events
// ─────────────────────────────────────────────

func (l *ChunkerLogger) ChunkStart(workerID, chunkID, urlCount int) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	
	workerColor := l.getWorkerColor(workerID)
	fmt.Printf("  %s %s %s\n",
		l.color(workerColor, fmt.Sprintf("[W%d]", workerID)),
		l.color(Cyan, fmt.Sprintf("▶ Starting chunk #%d", chunkID)),
		l.color(Dim, fmt.Sprintf("(%d URLs)", urlCount)))
}

func (l *ChunkerLogger) ChunkComplete(workerID, chunkID, pages, errors int, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	workerColor := l.getWorkerColor(workerID)
	
	statusIcon := l.color(BrightGreen, "✓")
	if errors > 0 {
		statusIcon = l.color(Yellow, "⚠")
	}
	
	fmt.Printf("  %s %s Chunk #%-3d │ %s pages │ %s errors │ %s\n",
		l.color(workerColor, fmt.Sprintf("[W%d]", workerID)),
		statusIcon,
		chunkID,
		l.color(BrightGreen, fmt.Sprintf("%3d", pages)),
		l.color(Yellow, fmt.Sprintf("%2d", errors)),
		l.color(Dim, duration.Round(time.Millisecond).String()))
}

// ─────────────────────────────────────────────
//  Page Events
// ─────────────────────────────────────────────

func (l *ChunkerLogger) PageScraped(workerID, chunkID int, url, title string, statusCode int) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	
	workerColor := l.getWorkerColor(workerID)
	statusColor := l.getStatusColor(statusCode)
	
	displayTitle := truncateStr(title, 30)
	if displayTitle == "" {
		displayTitle = "(no title)"
	}
	displayURL := truncateStr(url, 40)
	
	fmt.Printf("  %s %s %s %s\n",
		l.color(workerColor, fmt.Sprintf("[W%d:C%d]", workerID, chunkID)),
		l.color(statusColor, fmt.Sprintf("[%d]", statusCode)),
		l.color(White, displayTitle),
		l.color(Dim, displayURL))
}

func (l *ChunkerLogger) PageError(workerID, chunkID int, url string, err error) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	
	workerColor := l.getWorkerColor(workerID)
	displayURL := truncateStr(url, 40)
	errMsg := truncateStr(err.Error(), 30)
	
	fmt.Printf("  %s %s %s %s\n",
		l.color(workerColor, fmt.Sprintf("[W%d:C%d]", workerID, chunkID)),
		l.color(Red, "✗ ERR"),
		l.color(Dim, displayURL),
		l.color(Red, errMsg))
}

// ─────────────────────────────────────────────
//  Progress Bar
// ─────────────────────────────────────────────

func (l *ChunkerLogger) ProgressBar(p *Progress, final bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	total := atomic.LoadInt64(&p.TotalURLs)
	processed := atomic.LoadInt64(&p.ProcessedURLs)
	completedChunks := atomic.LoadInt32(&p.CompletedChunks)
	activeWorkers := atomic.LoadInt32(&p.ActiveWorkers)
	errors := atomic.LoadInt64(&p.TotalErrors)
	
	if total == 0 {
		return
	}
	
	percent := float64(processed) / float64(total) * 100
	elapsed := time.Since(p.StartTime)
	
	// Calculate ETA
	var eta string
	if processed > 0 && percent < 100 {
		remaining := float64(total-processed) / (float64(processed) / elapsed.Seconds())
		eta = time.Duration(remaining * float64(time.Second)).Round(time.Second).String()
	} else {
		eta = "--"
	}
	
	// Speed
	speed := float64(processed) / elapsed.Seconds()
	if elapsed.Seconds() == 0 {
		speed = 0
	}
	
	// Progress bar
	barWidth := 35
	filled := int(float64(barWidth) * float64(processed) / float64(total))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	
	// Build status line
	statusLine := fmt.Sprintf("\r  %s %s %s │ %s │ %s │ %s │ %s   ",
		l.color(Cyan, fmt.Sprintf("%5.1f%%", percent)),
		l.color(BrightBlue, bar),
		l.color(White, fmt.Sprintf("%d/%d", processed, total)),
		l.color(Green, fmt.Sprintf("C:%d/%d", completedChunks, p.TotalChunks)),
		l.color(Magenta, fmt.Sprintf("W:%d", activeWorkers)),
		l.color(Yellow, fmt.Sprintf("E:%d", errors)),
		l.color(Dim, fmt.Sprintf("%.1f/s ETA:%s", speed, eta)))
	
	fmt.Print(statusLine)
	
	if final || percent >= 100 {
		fmt.Println()
	}
}

// ─────────────────────────────────────────────
//  Summary
// ─────────────────────────────────────────────

func (l *ChunkerLogger) Summary(s Summary) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	fmt.Println()
	fmt.Println(l.color(BrightCyan, "  ╔═══════════════════════════════════════════════════════════════════════╗"))
	fmt.Println(l.color(BrightCyan, "  ║") + l.color(BrightGreen+Bold, "                      ✨ CHUNKED CRAWL COMPLETE ✨                       ") + l.color(BrightCyan, "║"))
	fmt.Println(l.color(BrightCyan, "  ╠═══════════════════════════════════════════════════════════════════════╣"))
	
	// Main stats
	fmt.Printf(l.color(BrightCyan, "  ║")+"  📄 Total Pages:      %-50s"+l.color(BrightCyan, "║")+"\n",
		l.color(BrightGreen+Bold, fmt.Sprintf("%d", s.TotalPages)))
	fmt.Printf(l.color(BrightCyan, "  ║")+"  ❌ Total Errors:     %-50s"+l.color(BrightCyan, "║")+"\n",
		l.color(Yellow, fmt.Sprintf("%d", s.TotalErrors)))
	fmt.Printf(l.color(BrightCyan, "  ║")+"  🧩 Chunks Processed: %-50s"+l.color(BrightCyan, "║")+"\n",
		l.color(Cyan, fmt.Sprintf("%d", s.TotalChunks)))
	fmt.Printf(l.color(BrightCyan, "  ║")+"  ⏱️  Total Duration:   %-50s"+l.color(BrightCyan, "║")+"\n",
		l.color(Cyan, s.Duration.Round(time.Millisecond).String()))
	fmt.Printf(l.color(BrightCyan, "  ║")+"  ⚡ Speed:            %-50s"+l.color(BrightCyan, "║")+"\n",
		l.color(Cyan, fmt.Sprintf("%.1f pages/sec", s.PagesPerSecond)))
	fmt.Printf(l.color(BrightCyan, "  ║")+"  📦 Total Size:       %-50s"+l.color(BrightCyan, "║")+"\n",
		l.color(Cyan, humanizeBytes(s.TotalSize)))
	
	fmt.Println(l.color(BrightCyan, "  ╠═══════════════════════════════════════════════════════════════════════╣"))
	
	// Chunk performance
	fmt.Println(l.color(BrightCyan, "  ║") + l.color(White+Bold, "  📊 CHUNK PERFORMANCE                                                  ") + l.color(BrightCyan, "║"))
	fmt.Println(l.color(BrightCyan, "  ╠═══════════════════════════════════════════════════════════════════════╣"))
	
	if s.FastestChunk > 0 {
		fmt.Printf(l.color(BrightCyan, "  ║")+"  🏆 Fastest: Chunk #%-3d %-46s"+l.color(BrightCyan, "║")+"\n",
			s.FastestChunk,
			l.color(Green, s.FastestDuration.Round(time.Millisecond).String()))
		fmt.Printf(l.color(BrightCyan, "  ║")+"  🐢 Slowest: Chunk #%-3d %-46s"+l.color(BrightCyan, "║")+"\n",
			s.SlowestChunk,
			l.color(Yellow, s.SlowestDuration.Round(time.Millisecond).String()))
	}
	
	fmt.Println(l.color(BrightCyan, "  ╠═══════════════════════════════════════════════════════════════════════╣"))
	
	// HTTP Status breakdown
	fmt.Println(l.color(BrightCyan, "  ║") + l.color(White+Bold, "  📈 HTTP STATUS BREAKDOWN                                              ") + l.color(BrightCyan, "║"))
	for code, count := range s.StatusCodes {
		emoji := statusEmoji(code)
		color := l.getStatusColor(code)
		barLen := count * 30 / s.TotalPages
		if barLen < 1 {
			barLen = 1
		}
		bar := strings.Repeat("█", barLen)
		fmt.Printf(l.color(BrightCyan, "  ║")+"     %s %s: %-5d %s%s"+l.color(BrightCyan, "║")+"\n",
			emoji,
			l.color(color, fmt.Sprintf("%d", code)),
			count,
			l.color(color, bar),
			strings.Repeat(" ", 50-barLen-10))
	}
	
	fmt.Println(l.color(BrightCyan, "  ╠═══════════════════════════════════════════════════════════════════════╣"))
	
	// Per-chunk breakdown (limited)
	fmt.Println(l.color(BrightCyan, "  ║") + l.color(White+Bold, "  🧩 CHUNK BREAKDOWN                                                    ") + l.color(BrightCyan, "║"))
	
	displayStats := s.WorkerStats
	if len(displayStats) > 10 {
		displayStats = displayStats[:10]
	}
	
	for _, ws := range displayStats {
		statusIcon := l.color(Green, "✓")
		if ws.Errors > 0 {
			statusIcon = l.color(Yellow, "⚠")
		}
		fmt.Printf(l.color(BrightCyan, "  ║")+"     %s Chunk #%-3d │ %s pages │ %s errors │ %s%s"+l.color(BrightCyan, "║")+"\n",
			statusIcon,
			ws.ChunkID,
			l.color(Green, fmt.Sprintf("%3d", ws.Pages)),
			l.color(Yellow, fmt.Sprintf("%2d", ws.Errors)),
			l.color(Dim, ws.Duration.Round(time.Millisecond).String()),
			strings.Repeat(" ", 25))
	}
	
	if len(s.WorkerStats) > 10 {
		fmt.Printf(l.color(BrightCyan, "  ║")+"     %s%s"+l.color(BrightCyan, "║")+"\n",
			l.color(Dim, fmt.Sprintf("... and %d more chunks", len(s.WorkerStats)-10)),
			strings.Repeat(" ", 43))
	}
	
	fmt.Println(l.color(BrightCyan, "  ╚═══════════════════════════════════════════════════════════════════════╝"))
	fmt.Println()
}

// ─────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────

func (l *ChunkerLogger) getWorkerColor(workerID int) string {
	colors := []string{BrightGreen, BrightYellow, BrightBlue, BrightMagenta, BrightCyan, Green, Yellow, Blue}
	return colors[(workerID-1)%len(colors)]
}

func (l *ChunkerLogger) getStatusColor(code int) string {
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

func truncateStr(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func humanizeBytes(b int64) string {
	if b == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB"}
	f := float64(b)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", b)
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

func statusEmoji(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "✅"
	case code >= 300 && code < 400:
		return "↗️"
	case code >= 400 && code < 500:
		return "⚠️"
	case code >= 500:
		return "🔴"
	default:
		return "❓"
	}
}
