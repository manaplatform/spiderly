package exclude

import (
	"fmt"
	"regexp"
	"strings"
)

// DefaultPatterns matches common non-product sections and keyword query terms.
var DefaultPatterns = []string{
	`(?i)/blog(/|$)`,
	`(?i)/news(/|$)`,
	`(?i)/category(/|$)`,
	`(?i)/abouts(/|$)`,
	`(?i)/contactus(/|$)`,
	`(?i)keyword`,
}

// CompilePatterns attempts to compile all provided regex strings and returns
// both the successfully parsed dataset and an error summarizing failures.
func CompilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	var failures []string
	for _, raw := range patterns {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		re, err := regexp.Compile(trimmed)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%q: %v", trimmed, err))
			continue
		}
		compiled = append(compiled, re)
	}
	if len(failures) > 0 {
		return compiled, fmt.Errorf("failed to compile exclude patterns: %s", strings.Join(failures, ", "))
	}
	return compiled, nil
}
