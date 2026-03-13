package report

type CrawlReport struct {
	Meta       Meta       `json:"meta"`
	Pages      []Page     `json:"pages"`
	Statistics Statistics `json:"statistics"`
}

type Meta struct {
	StartURL string `json:"start_url"`
}

type Statistics struct {
	TotalPages int `json:"total_pages"`
}

type Page struct {
	URL        string `json:"url"`
	Title      string `json:"title,omitempty"`
	StatusCode int    `json:"status_code"`
	Links      []Link `json:"links,omitempty"`
}

type Link struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
}
