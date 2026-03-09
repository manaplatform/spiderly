package httpclient

import (
	"github.com/gocolly/colly/v2"
	"net/http"
)

// CreateCustomHTTPClient initializes a custom HTTP client.
func CreateCustomHTTPClient() *http.Client {
	// Custom client configuration
	client := &http.Client{}
	return client
}

func NewCollectorWithCustomClient() *colly.Collector {
	// Create a custom HTTP client
	client := CreateCustomHTTPClient()

	// Create a new collector using the custom client
	c := colly.NewCollector()
	c.WithTransport(client.Transport)

	return c
}
