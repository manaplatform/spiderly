# Spiderly

## 1. Full Project Description
Spiderly is a high-performance web crawler and scraper built in Go. It utilizes powerful libraries such as `gocolly/colly` for efficient data extraction and `goquery` for HTML parsing, with capabilities ready for headless browser interactions via `chromedp`. The crawler is designed to scale and can perform deep site exploration using recursive crawling and XML sitemap parsing. It supports parallel chunked processing to efficiently handle large amounts of URLs using a multi-worker pool. Additionally, Spiderly features a real-time WebSocket dashboard for monitoring crawling progress, live logs, statistics, and extracted content on the fly.

## 2. Project Structure
```text
.
├── cmd/
│   └── main.go              # Entry point for the CLI application
├── internal/
│   ├── chunker/             # Parallel chunk processing and orchestration
│   ├── core/                # Core configuration and main run loop
│   ├── crawler/             # Gocolly-based web crawling engine
│   ├── models/              # Data structures (News, Sitemap, CrawlStats, WebSocketMessage)
│   ├── scraper/             # Extractor binding the crawler and the web dashboard
│   ├── sitemap/             # Sitemap discovery and XML parsing
│   └── ui/                  # UI components (now primarily the web dashboard)
├── go.mod                   # Go module dependencies
└── go.sum                   # Go module checksums
```

## 3. Installation
Ensure you have Go 1.25 or later installed on your system.

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd spiderly
   ```

2. Download and verify dependencies:
   ```bash
   go mod download
   go mod tidy
   ```

3. Build the binary:
   ```bash
   go build -o spiderly cmd/main.go
   ```

## 4. Commands
Spiderly provides a rich set of command-line flags to customize the crawling behavior. 

**Basic Options:**
* `-url <string>`: Target URL to crawl (required)
* `-pages <int>`: Maximum pages to scrape (default: 100)
* `-depth <int>`: Maximum crawl depth (default: 10)
* `-concurrency <int>`: Concurrent requests per worker (default: 5)
* `-timeout <duration>`: Request timeout (default: 30s)
* `-delay <duration>`: Delay between requests (default: 200ms)

**Chunker Options (Parallel Processing):**
* `-chunked`: Enable parallel chunked processing
* `-chunk-size <int>`: URLs per chunk (default: 50)
* `-workers <int>`: Number of parallel workers (default: 4)

**Output Options:**
* `-output <string>`: Path for JSON output file
* `-markdown <string>`: Path for Markdown output file

**Other Options:**
* `-recursive`: Force recursive crawl (skip sitemap)
* `-verbose`: Enable verbose logging
* `-no-color`: Disable colored output

## 5. Instructions

**Basic Crawling:**
To start crawling a website with the default settings:
```bash
./spiderly -url https://example.com
```

**High-Performance Parallel Crawling:**
Speed up the scraping process by dividing URLs into chunks and using multiple parallel workers:
```bash
./spiderly -url https://example.com -pages 500 -chunked -chunk-size 100 -workers 8
```

**Exporting Results:**
Save the extracted data into JSON or Markdown formats for further analysis or documentation:
```bash
./spiderly -url https://example.com -output results.json -markdown report.md -verbose
```

**Monitoring via Web Dashboard:**
When Spiderly starts crawling, it automatically spins up a real-time WebSocket dashboard. Look for the console output specifying the dashboard port (e.g., `Dashboard running at: http://localhost:<port>`). Open this URL in your browser to view live crawling statistics, discovered links, and extracted page models in real-time.