package cli

import "time"

type CrawlOptions struct {
	StartURL           string
	AllowedDomain      string
	MaxDepth           int
	MaxPages           int
	Concurrency        int
	Timeout            time.Duration
	UserAgent          string
	RespectRobots      bool
	IncludeSubdomains  bool
	ProductMode        bool
	ProductPattern     string
	NewsMode           bool
	NewsPattern        string
	CategoryPattern    string
	ExcludePatterns    []string
	IncludePatterns    []string
	OutputJSON         string
	OutputMarkdown     string
	ReportTitle        string
	ShowSummary        bool
	PrettyJSON         bool
	FailOnHTTPError    bool
	FailOnBrokenLinks  bool
	Headers            []string
	Cookies            []string
	Delay              time.Duration
	InsecureSkipVerify bool
	RenderJS           bool
	SitemapURL         string
}
