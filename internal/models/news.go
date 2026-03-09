package models

import "time"

// News represents a single news article
type News struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Summary     string   `json:"summary"`
	URL         string   `json:"url"`
	ImageURL    string   `json:"image_url"`
	Author      string   `json:"author"`
	PublishedAt string   `json:"published_at"`
	ScrapedAt   time.Time `json:"scraped_at"`
	Tags        []string `json:"tags"`
}

// Link represents a discovered link on a page
type Link struct {
	URL   string `json:"url"`
	Text  string `json:"text"`
	Depth int    `json:"depth"`
}

// CrawlResult holds the result of crawling a page
type CrawlResult struct {
	URL         string        `json:"url"`
	News        *News         `json:"news"`
	Links       []Link        `json:"links"`
	Error       error         `json:"-"`
	ErrorMsg    string        `json:"error_msg,omitempty"`
	StatusCode  int           `json:"status_code"`
	ElapsedTime time.Duration `json:"elapsed_time"`
}

// WSMessage is a WebSocket message sent to the frontend
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// LogEntry represents a log message
type LogEntry struct {
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// StatsPayload holds crawling statistics
type StatsPayload struct {
	TotalPages  int    `json:"total_pages"`
	TotalNews   int    `json:"total_news"`
	TotalLinks  int    `json:"total_links"`
	Errors      int    `json:"errors"`
	ElapsedTime string `json:"elapsed_time"`
	Status      string `json:"status"`
}

// ProgressPayload holds progress info
type ProgressPayload struct {
	CurrentURL string  `json:"current_url"`
	Progress   float64 `json:"progress"`
	PagesDone  int     `json:"pages_done"`
	PagesTotal int     `json:"pages_total"`
}
