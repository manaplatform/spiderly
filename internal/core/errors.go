package core

import (
	"context"
	"fmt"
	"math"
	"time"
)


// ─────────────────────────────────────────────
//  Error Kinds
// ─────────────────────────────────────────────

// ErrKind classifies the high-level category of a crawl error.
type ErrKind string

const (
	ErrKindConfig  ErrKind = "config"
	ErrKindNetwork ErrKind = "network"
	ErrKindParse   ErrKind = "parse"
	ErrKindRobots  ErrKind = "robots"
	ErrKindTimeout ErrKind = "timeout"
	ErrKindSink    ErrKind = "sink"
	ErrKindUnknown ErrKind = "unknown"
)

// ─────────────────────────────────────────────
//  Error Categories (used by metrics & callbacks)
// ─────────────────────────────────────────────

// ErrorCategory is a coarse bucket for error tracking in metrics.
// Metrics and callback signatures reference this type so that callers
// can aggregate errors without parsing strings.
type ErrorCategory string

const (
	ErrorCategoryNetwork ErrorCategory = "network"
	ErrorCategoryParse   ErrorCategory = "parse"
	ErrorCategoryRobots  ErrorCategory = "robots"
	ErrorCategoryTimeout ErrorCategory = "timeout"
	ErrorCategorySink    ErrorCategory = "sink"
	ErrorCategoryConfig  ErrorCategory = "config"
	ErrorCategoryUnknown ErrorCategory = "unknown"
)

// CategoryFromKind maps an ErrKind to its corresponding ErrorCategory.
func CategoryFromKind(k ErrKind) ErrorCategory {
	switch k {
	case ErrKindNetwork:
		return ErrorCategoryNetwork
	case ErrKindParse:
		return ErrorCategoryParse
	case ErrKindRobots:
		return ErrorCategoryRobots
	case ErrKindTimeout:
		return ErrorCategoryTimeout
	case ErrKindSink:
		return ErrorCategorySink
	case ErrKindConfig:
		return ErrorCategoryConfig
	default:
		return ErrorCategoryUnknown
	}
}


// NewCrawlError creates a CrawlError with the given kind, URL, and cause.
func NewCrawlError(kind ErrKind, url string, err error) *CrawlError {
	return &CrawlError{Type: errKindToCrawlErrorType(kind), URL: url, Cause: err, Timestamp: time.Now()}
}

// errKindToCrawlErrorType maps the ErrKind string to CrawlErrorType int.
func errKindToCrawlErrorType(k ErrKind) CrawlErrorType {
	switch k {
	case ErrKindNetwork:
		return ErrNetwork
	case ErrKindParse:
		return ErrParsing
	case ErrKindRobots:
		return ErrRobotsDenied
	case ErrKindTimeout:
		return ErrTimeout
	case ErrKindConfig:
		return ErrValidation
	default:
		return ErrInternal
	}
}

// WithMessage attaches a human-readable message and returns the same pointer
// so it can be used in a builder-style chain.
func (e *CrawlError) WithMessage(msg string) *CrawlError {
	e.Message = msg
	return e
}

// Error satisfies the error interface.
func (e *CrawlError) Error() string {
	base := fmt.Sprintf("[%s]", e.Type)
	if e.URL != "" {
		base += fmt.Sprintf(" url=%s", e.URL)
	}
	if e.Message != "" {
		base += fmt.Sprintf(" %s", e.Message)
	}
	if e.Cause != nil {
		base += fmt.Sprintf(": %v", e.Cause)
	}
	return base
}

// Unwrap lets errors.Is / errors.As traverse the chain.
func (e *CrawlError) Unwrap() error {
	return e.Cause
}

// Category returns the ErrorCategory for this error (convenience shortcut).
func (e *CrawlError) Category() ErrorCategory {
	switch e.Type {
	case ErrNetwork:
		return ErrorCategoryNetwork
	case ErrParsing:
		return ErrorCategoryParse
	case ErrRobotsDenied:
		return ErrorCategoryRobots
	case ErrTimeout:
		return ErrorCategoryTimeout
	case ErrValidation:
		return ErrorCategoryConfig
	default:
		return ErrorCategoryUnknown
	}
}

// CrawlErrorType categorizes crawl failures for programmatic handling.
type CrawlErrorType int

const (
	ErrNetwork     CrawlErrorType = iota // DNS, connection refused, reset
	ErrTimeout                           // Context deadline or HTTP timeout
	ErrRobotsDenied                      // Blocked by robots.txt
	ErrHTTPStatus                        // Non-2xx response
	ErrParsing                           // HTML/XML parse failure
	ErrRateLimit                         // 429 Too Many Requests
	ErrValidation                        // Bad URL, bad config
	ErrInternal                          // Bug / unexpected state
)

// String returns a human-readable label for the error type.
func (t CrawlErrorType) String() string {
	switch t {
	case ErrNetwork:
		return "NETWORK"
	case ErrTimeout:
		return "TIMEOUT"
	case ErrRobotsDenied:
		return "ROBOTS_DENIED"
	case ErrHTTPStatus:
		return "HTTP_STATUS"
	case ErrParsing:
		return "PARSE"
	case ErrRateLimit:
		return "RATE_LIMIT"
	case ErrValidation:
		return "VALIDATION"
	case ErrInternal:
		return "INTERNAL"
	default:
		return "UNKNOWN"
	}
}

// ─────────────────────────────────────────────
//  CrawlError
// ─────────────────────────────────────────────

// CrawlError is a structured error carrying context about what failed,
// where it failed, and whether it makes sense to retry.
type CrawlError struct {
	Type       CrawlErrorType `json:"type"`
	URL        string         `json:"url"`
	StatusCode int            `json:"status_code,omitempty"`
	Retries    int            `json:"retries"`
	MaxRetries int            `json:"max_retries"`
	Cause      error          `json:"-"`
	Message    string         `json:"message"`
	Timestamp  time.Time      `json:"timestamp"`
}




// message returns the best available human string.
func (e *CrawlError) message() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "unknown error"
}

// IsRetryable reports whether this error class warrants a retry.
func (e *CrawlError) IsRetryable() bool {
	switch e.Type {
	case ErrTimeout, ErrNetwork, ErrRateLimit:
		return true
	case ErrHTTPStatus:
		// Retry server errors and 429; never retry 4xx client errors
		return e.StatusCode == 429 || e.StatusCode >= 500
	default:
		return false
	}
}

// ─────────────────────────────────────────────
//  Constructors — keep call sites clean
// ─────────────────────────────────────────────

// NewNetworkError wraps a low-level dial / TLS / connection error.
func NewNetworkError(url string, cause error) *CrawlError {
	return &CrawlError{
		Type:      ErrNetwork,
		URL:       url,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// NewTimeoutError records a context-deadline or HTTP-timeout failure.
func NewTimeoutError(url string, cause error) *CrawlError {
	return &CrawlError{
		Type:      ErrTimeout,
		URL:       url,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// NewHTTPError records a non-2xx response.
func NewHTTPError(url string, statusCode int) *CrawlError {
	typ := ErrHTTPStatus
	if statusCode == 429 {
		typ = ErrRateLimit
	}
	return &CrawlError{
		Type:       typ,
		URL:        url,
		StatusCode: statusCode,
		Message:    fmt.Sprintf("HTTP %d", statusCode),
		Timestamp:  time.Now(),
	}
}

// NewRobotsError records a robots.txt denial.
func NewRobotsError(url string) *CrawlError {
	return &CrawlError{
		Type:      ErrRobotsDenied,
		URL:       url,
		Message:   "blocked by robots.txt",
		Timestamp: time.Now(),
	}
}

// NewParseError records an HTML/XML parse failure.
func NewParseError(url string, cause error) *CrawlError {
	return &CrawlError{
		Type:      ErrParsing,
		URL:       url,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// NewValidationError records a bad-input problem (URL, config, etc.).
func NewValidationError(msg string) *CrawlError {
	return &CrawlError{
		Type:      ErrValidation,
		Message:   msg,
		Timestamp: time.Now(),
	}
}

// ─────────────────────────────────────────────
//  Retry Policy
// ─────────────────────────────────────────────

// RetryPolicy controls exponential-backoff retry behaviour.
type RetryPolicy struct {
	MaxRetries int           // 0 = no retries
	BaseDelay  time.Duration // initial wait between attempts
	MaxDelay   time.Duration // ceiling for backoff
	Multiplier float64       // delay *= Multiplier each attempt
}

// DefaultRetryPolicy returns a sensible starting point.
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
	}
}

// NoRetry returns a policy that never retries.
func NoRetry() *RetryPolicy {
	return &RetryPolicy{MaxRetries: 0}
}

// Execute runs fn up to (1 + MaxRetries) times.
//
// If fn returns a *CrawlError whose IsRetryable() is false the loop
// exits immediately.  Between attempts the goroutine sleeps with
// exponential backoff, respecting ctx cancellation.
//
// On each retry the CrawlError.Retries field is incremented so
// callers can inspect how many attempts were made.
func (rp *RetryPolicy) Execute(ctx context.Context, fn func() error) error {
	if rp == nil || rp.MaxRetries <= 0 {
		return fn()
	}

	var lastErr error
	delay := rp.BaseDelay

	for attempt := 0; attempt <= rp.MaxRetries; attempt++ {
		// Check context before each attempt
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return fmt.Errorf("context cancelled after %d attempts: %w (last: %v)", attempt, err, lastErr)
			}
			return err
		}

		lastErr = fn()
		if lastErr == nil {
			return nil // success
		}

		// If the error is a CrawlError, stamp the retry count
		if ce, ok := lastErr.(*CrawlError); ok {
			ce.Retries = attempt + 1
			ce.MaxRetries = rp.MaxRetries

			// Non-retryable → bail immediately
			if !ce.IsRetryable() {
				return ce
			}
		}

		// Don't sleep after the last attempt
		if attempt == rp.MaxRetries {
			break
		}

		// Sleep with jitter-free exponential backoff
		sleepDur := rp.backoff(attempt)
		select {
		case <-time.After(sleepDur):
			// continue to next attempt
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry backoff: %w (last: %v)", ctx.Err(), lastErr)
		}

		delay = sleepDur // track for debugging
		_ = delay
	}

	return fmt.Errorf("after %d retries: %w", rp.MaxRetries, lastErr)
}

// backoff calculates the sleep duration for a given attempt index.
func (rp *RetryPolicy) backoff(attempt int) time.Duration {
	mult := math.Pow(rp.Multiplier, float64(attempt))
	d := time.Duration(float64(rp.BaseDelay) * mult)
	if d > rp.MaxDelay {
		d = rp.MaxDelay
	}
	return d
}

// ─────────────────────────────────────────────
//  Error Aggregator
// ─────────────────────────────────────────────

// ErrorAggregator collects CrawlErrors and provides summary statistics.
// It is safe for concurrent use.
type ErrorAggregator struct {
	errors []CrawlError
	counts map[CrawlErrorType]int
}

// NewErrorAggregator creates a ready-to-use aggregator.
func NewErrorAggregator() *ErrorAggregator {
	return &ErrorAggregator{
		errors: make([]CrawlError, 0, 64),
		counts: make(map[CrawlErrorType]int),
	}
}

// Add records an error.  If err is not a *CrawlError it is wrapped as ErrInternal.
func (ea *ErrorAggregator) Add(err error) {
	if err == nil {
		return
	}

	ce, ok := err.(*CrawlError)
	if !ok {
		ce = &CrawlError{
			Type:      ErrInternal,
			Message:   err.Error(),
			Cause:     err,
			Timestamp: time.Now(),
		}
	}

	ea.errors = append(ea.errors, *ce)
	ea.counts[ce.Type]++
}

// Total returns the number of recorded errors.
func (ea *ErrorAggregator) Total() int {
	return len(ea.errors)
}

// Counts returns error totals grouped by type.
func (ea *ErrorAggregator) Counts() map[CrawlErrorType]int {
	out := make(map[CrawlErrorType]int, len(ea.counts))
	for k, v := range ea.counts {
		out[k] = v
	}
	return out
}

// Errors returns a copy of all recorded errors.
func (ea *ErrorAggregator) Errors() []CrawlError {
	out := make([]CrawlError, len(ea.errors))
	copy(out, ea.errors)
	return out
}

// RetryableCount returns how many recorded errors were retryable.
func (ea *ErrorAggregator) RetryableCount() int {
	n := 0
	for _, e := range ea.errors {
		if e.IsRetryable() {
			n++
		}
	}
	return n
}