# 🕷️ Spiderly

> A high-performance, concurrent web crawler and scraper built in Go.

Spiderly is a fast, flexible command-line web crawler designed for deep site exploration at scale. It leverages [Colly](https://github.com/gocolly/colly) for efficient crawling, [GoQuery](https://github.com/PuerkitoBio/goquery) for HTML parsing, and supports headless browser interactions via [ChromeDP](https://github.com/chromedp/chromedp). Spiderly can discover pages through XML sitemap parsing or recursive link following, process URLs in parallel chunks with a multi-worker pool, and export structured results to JSON or Markdown — all with a rich, colorized console experience.

---

## ✨ Features

### Crawling & discovery
- Guide Spiderly with sitemap seeds or recursive link-following while tuning depth, timeout, and delay to suit each target URL. Optional priority and URL filters plus the `-recursive` override help you balance auto-discovery with deep traversal. [`cmd/main.go:23-52`]

### Parallel chunked processing
- Chunked mode splits the crawl into batches so multiple workers (`-chunk-size`, `-workers`) can fetch pages concurrently while respecting per-worker concurrency, delays, and timeouts. [`cmd/main.go:36-85`]

### Product intelligence
- Enable `-product-mode` (or provide `-product-pattern`) to focus on ecommerce listings. Product mode auto-promotes chunked processing, raises the default page/chunk/worker counts, and lets you customize the regex that identifies product URLs. [`cmd/main.go:43-118`]
- Markdown reports surface product summaries, price distributions, stock counts, and per-product metadata whenever product data is available, while JSON output and the console summary keep crawl stats, status codes, and keyword insights organized for automation. [`cmd/main.go:269-405`][`cmd/main.go:500-580`]

### Output & observability
- Save indented JSON or illustrated Markdown reports, and rely on Spiderly's summary box, verbose logging, and ANSI-safe mode to understand throughput, HTTP statuses, and any errors that arise during the run. [`cmd/main.go:269-365`][`cmd/main.go:539-670`]

---

## 📦 Installation

### Prerequisites

- [Go](https://go.dev/dl/) **1.25** or later

### From source

```bash
# Clone the repository
git clone https://github.com/your-org/spiderly.git
cd spiderly

# Download dependencies
go mod download

# Build the binary
go build -o spiderly cmd/main.go
```

### Using `go install`

```bash
go install spiderly/cmd@latest
```

---

## 🚀 Usage

```bash
spiderly -url <target> [options]
```

### CLI Flags

| Flag                 | Default       | Description                                                              |
|---                   |---            |---                                                                        |
| `-url`               | *(required)*  | Target URL to crawl                                                       |
| `-pages`             | `100`         | Maximum number of pages to scrape                                         |
| `-depth`             | `10`          | Maximum crawl depth                                                       |
| `-concurrency`       | `5`           | Concurrent requests per worker                                            |
| `-timeout`           | `30s`         | Request timeout (Go duration, e.g. `10s`, `1m`)                           |
| `-delay`             | `200ms`       | Delay between requests (Go duration)                                      |
| `-chunked`           | `false`       | Enable parallel chunked processing                                        |
| `-chunk-size`        | `50`          | Number of URLs per chunk                                                  |
| `-workers`           | `4`           | Number of parallel chunk workers                                          |
| `-sitemap`           | —             | Direct sitemap URL (skip auto-discovery)                                  |
| `-min-priority`      | `0`           | Minimum sitemap priority filter (0.0 - 1.0)                               |
| `-url-pattern`       | —             | Regex to filter sitemap URLs                                              |
| `-product-mode`      | `false`       | Enable product-only crawl mode (auto-enables chunked)                     |
| `-product-pattern`   | —             | Custom regex for product URLs                                             |
| `-output`            | —             | File path for JSON output                                                 |
| `-markdown`          | —             | File path for Markdown report output                                      |
| `-recursive`         | `false`       | Force recursive crawl (skip sitemap discovery)                            |
| `-verbose`           | `false`       | Enable verbose / debug logging                                            |
| `-no-color`          | `false`       | Disable colored terminal output                                           |
### Examples

**Basic crawl with defaults:**

```bash
spiderly -url https://example.com
```

**High-throughput parallel crawl:**

```bash
spiderly -url https://example.com -pages 500 -chunked -chunk-size 100 -workers 8
```

**Export results to JSON and Markdown:**

```bash
spiderly -url https://example.com -output results.json -markdown report.md -verbose
```

**Recursive crawl with custom depth and timeout:**

```bash
spiderly -url https://example.com -recursive -depth 5 -timeout 15s -concurrency 10
```

---

## 🏗️ Project Structure

```text
.
├── cmd/
│   └── main.go              # CLI entry point — flag parsing, orchestration, output saving
├── internal/
│   ├── chunker/
│   │   └── chunker.go       # Parallel chunk processing engine (worker pool)
│   ├── core/
│   │   └── core.go          # Core configuration, main run loop, console UI
│   ├── crawler/              # Colly-based web crawling engine
│   ├── models/               # Shared data structures (ScrapedPage, CrawlStats, etc.)
│   ├── scraper/              # Extraction layer binding crawler to data pipeline
│   ├── sitemap/              # Sitemap discovery and XML parsing
│   └── ui/                   # Terminal UI components
├── go.mod                    # Go module definition (module spiderly, go 1.25.7)
├── go.sum                    # Dependency checksums
└── README.md                 # This file
```

| Path | Description |
|---|---|
| `cmd/main.go` | Application entry point (507 lines). Parses all CLI flags, initialises the `Core`, triggers the crawl, and writes JSON / Markdown output files. |
| `internal/chunker/` | Parallel chunk processor (534 lines). Splits a URL list into fixed-size chunks and dispatches them to a configurable pool of workers for concurrent scraping. |
| `internal/core/` | Core orchestrator (944 lines). Holds the `CoreConfig`, manages the crawl lifecycle, coordinates the crawler and chunker, and renders the rich colorized console summary. |
| `internal/crawler/` | Colly-powered crawling engine. Handles HTTP requests, link extraction, and robots.txt compliance. |
| `internal/models/` | Shared data models — `ScrapedPage`, `CrawlStats`, sitemap entries, and WebSocket message types. |
| `internal/sitemap/` | Sitemap discovery — fetches and parses XML sitemaps to seed the URL queue. |
| `internal/scraper/` | Extraction glue layer connecting the crawler output to the data pipeline and optional dashboard. |
| `internal/ui/` | Terminal and web dashboard UI components for real-time monitoring. |
| `go.mod` / `go.sum` | Go module files tracking dependencies (Colly, GoQuery, ChromeDP, Lipgloss, and more). |

---

## ⚙️ Configuration

### Product mode & URL filters
When `-product-mode` is enabled (or a custom `-product-pattern` is provided), Spiderly auto-enables chunked processing, increases default page, chunk, and worker counts, and uses a regex to identify product pages. You can also supply a direct sitemap (`-sitemap`), filter by minimum priority (`-min-priority`), or apply a URL filter (`-url-pattern`), or force recursive link-following with `-recursive`. [`cmd/main.go:44-51`][`cmd/main.go:68-96`]

```bash
# Crawl products from sitemap with custom pattern and save JSON
spiderly -url https://example.com -product-mode -product-pattern "/product/" -output products.json
```

### Chunked processing
Split large crawls into batches of `-chunk-size` URLs processed concurrently across `-workers`. Each chunk worker still respects `-concurrency`, `-delay`, and `-timeout` to maintain throughput and politeness. [`cmd/main.go:36-85`]

```bash
# Process 1000 pages in chunks of 200 across 5 workers
spiderly -url https://example.com -pages 1000 -chunked -chunk-size 200 -workers 5
```

### Output formats & logging
Use `-output` for JSON export or `-markdown` for a rich Markdown report. Both can be combined. [`cmd/main.go:269-365`]

| Format     | Flag                | Description                                                               |
|---         |---                  |---                                                                         |
| **JSON**   | `-output results.json`   | Pretty-printed JSON array of scraped pages.                              |
| **Markdown**| `-markdown report.md`  | Human-readable Markdown report with stats, tables, and per-page details.  |

### Verbosity & colors
- `-verbose` prints detailed per-request logs.
- `-no-color` removes ANSI colors for compatibility with text outputs. [`cmd/main.go:31-33`]

---

## 🤝 Contributing

Contributions are welcome! Here's how to get started:

1. **Fork** the repository and create a new branch from `main`.
2. **Make your changes** — please keep commits focused and well-described.
3. **Run tests** and ensure the build passes:
   ```bash
   go build ./...
   go vet ./...
   ```
4. **Open a Pull Request** with a clear description of what you changed and why.

### Guidelines

- Follow standard Go conventions and formatting (`gofmt` / `goimports`).
- Keep public API changes backward-compatible where possible.
- Add or update documentation for any new flags or features.
- Be respectful and constructive in reviews and discussions.

---

## 📄 License

This project is licensed under the **MIT License**. See the [LICENSE](LICENSE) file for details.

---


*Built with ❤️ in Go — happy crawling!*
