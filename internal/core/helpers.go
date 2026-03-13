package core

import (
	"fmt"
	"strings"
)

// ─────────────────────────────────────────────
//  String Helpers
// ─────────────────────────────────────────────

// truncateString trims whitespace and truncates to maxLen with ellipsis.
func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// ─────────────────────────────────────────────
//  Byte Formatting
// ─────────────────────────────────────────────

// humanizeBytes converts a byte count into a human-readable string
// (e.g., 1536 → "1.5 KB", 0 → "0 B").
func humanizeBytes(b int64) string {
	if b == 0 {
		return "0 B"
	}

	const unit = 1024
	units := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(b)
	i := 0

	for f >= unit && i < len(units)-1 {
		f /= unit
		i++
	}

	if i == 0 {
		return fmt.Sprintf("%d B", b)
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

// ─────────────────────────────────────────────
//  HTTP Status Helpers
// ─────────────────────────────────────────────

// statusEmoji returns a visual emoji indicator for an HTTP status code.
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

// statusCategory returns a human-readable category for an HTTP status code.
func statusCategory(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "success"
	case code >= 300 && code < 400:
		return "redirect"
	case code >= 400 && code < 500:
		return "client_error"
	case code >= 500:
		return "server_error"
	default:
		return "unknown"
	}
}

// ─────────────────────────────────────────────
//  Slice Helpers
// ─────────────────────────────────────────────

// uniqueStrings deduplicates a string slice while preserving order.
func uniqueStrings(input []string) []string {
	seen := make(map[string]struct{}, len(input))
	result := make([]string, 0, len(input))

	for _, s := range input {
		if _, exists := seen[s]; !exists {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}

// containsString checks if a string slice contains a given value.
func containsString(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────
//  Numeric Helpers
// ─────────────────────────────────────────────

// clampInt constrains val to the range [min, max].
func clampInt(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// maxInt returns the larger of two ints.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// minInt returns the smaller of two ints.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}