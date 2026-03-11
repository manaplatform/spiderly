# 🕷️ Spiderly

> A high-performance, concurrent web crawler and scraper built in Go.

Spiderly is a fast, flexible command-line web crawler designed for deep site exploration at scale. It leverages [Colly](https://github.com/gocolly/colly) for efficient crawling, [GoQuery](https://github.com/PuerkitoBio/goquery) for HTML parsing, and supports headless browser interactions via [ChromeDP](https://github.com/chromedp/chromedp). Spiderly can discover pages through XML sitemap parsing or recursive link following, process URLs in parallel chunks with a multi-worker pool, and export structured results to JSON or Markdown — all with a rich, colorized console experience.

---

## ✨ Features

- **Deep recursive crawling** — explore entire websites with configurable depth and page limits
- **Sitemap-aware discovery** — automatically parses XML sitemaps; skip with `-recursive` for pure link-following
- **Parallel chunked processing** — split URL lists into chunks and process them across multiple concurrent workers for maximum throughput
- **JSON export** — save structured crawl results to a JSON file for programmatic consumption
- **Markdown export** — generate polished Markdown crawl reports with statistics, status code breakdowns, and per-page details
- **Configurable concurrency** — fine-tune concurrent requests per worker, request timeouts, and inter-request delays
- **Rich console output** — colorized, emoji-enhanced terminal summaries with real-time progress; disable colors with `--no-color`
- **Verbose logging** — toggle detailed request-level logging for debugging
- **Headless browser support** — built-in ChromeDP integration for JavaScript-rendered pages

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

| Flag | Default | Description |
|---|---|---|
| `-url` | *(required)* | Target URL to crawl |
| `-pages` | `100` | Maximum number of pages to scrape |
| `-depth` | `10` | Maximum crawl depth |
| `-concurrency` | `5` | Concurrent requests per worker |
| `-timeout` | `30s` | Request timeout (Go duration, e.g. `10s`, `1m`) |
| `-delay` | `200ms` | Delay between requests (Go duration) |
| `-chunked` | `false` | Enable parallel chunked processing |
| `-chunk-size` | `50` | Number of URLs per chunk |
| `-workers` | `4` | Number of parallel chunk workers |
| `-output` | — | File path for JSON output |
| `-markdown` | — | File path for Markdown report output |
| `-recursive` | `false` | Force recursive crawl (skip sitemap discovery) |
| `-verbose` | `false` | Enable verbose / debug logging |
| `-no-color` | `false` | Disable colored terminal output |

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

### Chunked Processing

When `-chunked` is enabled, Spiderly divides the discovered URL list into batches of `-chunk-size` URLs and processes them across `-workers` parallel workers. Each worker maintains its own Colly collector with the configured `-concurrency`, `-delay`, and `-timeout` values. This dramatically speeds up large crawls while keeping per-worker resource usage predictable.

```bash
# Process 1000 pages in chunks of 200 across 5 workers
spiderly -url https://example.com -pages 1000 -chunked -chunk-size 200 -workers 5
```

### Recursive Mode

By default Spiderly attempts to discover pages via the target site's XML sitemap. Pass `-recursive` to skip sitemap discovery entirely and instead follow links found on each page up to `-depth` levels deep.

```bash
spiderly -url https://example.com -recursive -depth 3
```

### Output Formats

| Format | Flag | Description |
|---|---|---|
| **JSON** | `-output results.json` | Pretty-printed JSON array of all scraped pages, including URL, title, meta tags, content length, load time, status code, and depth. |
| **Markdown** | `-markdown report.md` | Human-readable crawl report with summary statistics, HTTP status code distribution, keyword frequency, and per-page details. |

Both flags can be used together in a single run.

### Verbosity & Colors

- `-verbose` turns on detailed per-request logging so you can see every URL as it is fetched and any errors encountered.
- `-no-color` strips ANSI escape codes from all terminal output, useful when piping to a file or running in CI environments.

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

<p align="center">
  <em>Built with ❤️ in Go — happy crawling!</em>
</p>
