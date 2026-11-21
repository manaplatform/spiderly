
<p align="center">
  <img src="logo.png" alt="Spiderly Logo" width="300">
</p>

# Spiderly

> A high-performance, concurrent web crawler and scraper built in Go.

Spiderly is a fast, flexible command-line web crawler designed for deep site exploration at scale. It leverages [Colly](https://github.com/gocolly/colly) for efficient crawling, [GoQuery](https://github.com/PuerkitoBio/goquery) for HTML parsing, and supports headless browser interactions via [ChromeDP](https://github.com/chromedp/chromedp). Spiderly can discover pages through XML sitemap parsing or recursive link following, process URLs in parallel chunks with a multi-worker pool, and export structured results to JSON or Markdown — all with a rich, colorized console experience.

---

## ✨ Features

### Crawling & discovery
- Guide Spiderly with sitemap seeds or recursive link-following while tuning depth, timeout, and delay to suit each target URL. Full recursive crawl tracks parent-to-child lineage, discovers links continuously, and respects `robots.txt` and depth limits. Optional priority and URL filters plus the `-r` / `--force-recursive` override help you balance auto-discovery with deep traversal.

### Smart Hierarchy & Site Understanding
- Preserves the full crawl tree (`parent_url` → `child_urls`), mapping parent-child relationships and site structure.
- Multi-signal page classifier categorizes and segments pages via schema.org `BreadcrumbList`, HTML breadcrumbs, URL path decomposition, and parent inheritance.
- Visual ASCII/Unicode site tree generation for immediate structural understanding in console and Markdown.

### Structured Data Extraction (JSON-LD & Schema.org)
- Automatically parses and normalizes JSON-LD structured data on all pages (single objects, arrays, and `@graph` containers).
- Extracts and standardizes:
  - Product `name`, `description`, `brand`, `sku`, `gtin`, `mpn`, `productID`, `isbn`
  - `offers` and `aggregateOffer`: price normalization (numbers and currency strings), currency codes, and availability statuses (`InStock`, `OutOfStock`, `PreOrder`, etc.)
  - `aggregateRating`: rating values and review counts
  - `BreadcrumbList`: complete category hierarchy
  - `article` / `newsArticle`: headlines, author, and publication dates

### Smart Sorting & Filtering Engine
- Post-crawl query engine supports filtering by category, price range, in-stock availability, product existence, depth, or keywords.
- Flexible sorting by `hierarchy` (DFS tree order), `category`, `price`, `price-desc`, `title`, `depth`, `date`, or `rating`.
- Generates clean, executive-ready Markdown reports with visual ASCII trees, category breakdown tables, and structured product catalogs, as well as full-fidelity JSON.

### Parallel chunked processing
- Chunked mode splits the crawl into batches so multiple workers (`--chunk-size`, `--max-workers`) can fetch pages concurrently while respecting per-worker concurrency, delays, and timeouts.

### Proxy support
- Route crawl traffic through one or more HTTP proxies via `--proxy` or `SPIDERLY_PROXY`. In chunked mode, workers receive sticky proxy assignments (worker index modulo proxy list).

### Product intelligence
- Enable `-product-mode` (or provide `-product-pattern`) to focus on ecommerce listings. Product mode auto-promotes chunked processing, raises the default page/chunk/worker counts, and lets you customize the regex that identifies product URLs. [`cmd/main.go:43-118`]
- Markdown reports surface product summaries, price distributions, stock counts, and per-product metadata whenever product data is available, while JSON output and the console summary keep crawl stats, status codes, and keyword insights organized for automation. [`cmd/main.go:269-405`][`cmd/main.go:500-580`]

### News intelligence
- Enable `--news-mode` (or provide `--news-pattern`) to focus on article/news pages. News mode prioritizes news-like sitemaps, supports custom sitemap keyword hints via `--news-sitemaps`, and extracts headline/author/publish date/tags using DOM + meta tag parsing (no JSON-LD path).

### Output & observability
- Save indented JSON or illustrated Markdown reports, and rely on Spiderly's summary box, verbose logging, and ANSI-safe mode to understand throughput, HTTP statuses, and any errors that arise during the run. [`cmd/main.go:269-365`][`cmd/main.go:539-670`]

---

## 📦 Installation

### Prerequisites

- [Go](https://go.dev/dl/) **1.25** or later

### From source

```bash
# Clone the repository
git clone https://github.com/manaplatform/spiderly
cd spiderly

# Download dependencies
go mod download

# Build the binary
go build -o spiderly cmd/spiderly/main.go
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
| `--max-workers`      | `4`           | Number of parallel chunk workers (new crawl subcommand flag)              |
| `--proxy`            | —             | Proxy URL(s), supports repeated or comma-separated values                 |
| `-sitemap`           | —             | Direct sitemap URL (skip auto-discovery)                                  |
| `-min-priority`      | `0`           | Minimum sitemap priority filter (0.0 - 1.0)                               |
| `-url-pattern`       | —             | Regex to filter sitemap URLs                                              |
| `-product-mode`      | `false`       | Enable product-only crawl mode (auto-enables chunked)                     |
| `-product-pattern`   | —             | Custom regex for product URLs                                             |
| `--news-mode`        | `false`       | Enable news/article extraction mode                                       |
| `--news-pattern`     | —             | Custom regex for news/article URLs                                        |
| `--news-sitemaps`    | —             | Preferred sitemap keywords for news mode (e.g. `news,press`)             |
| `-output`, `-o`      | `crawl.json`  | File path for JSON output                                                 |
| `-markdown`, `-m`    | —             | File path for hierarchical Markdown report                                |
| `-r`, `--force-recursive` | `false`   | Force recursive crawl (skip sitemap discovery)                            |
| `--sort-by`          | `hierarchy`   | Sort pages: `hierarchy`, `category`, `price`, `price-desc`, `title`, `depth`, `date`, `rating` |
| `--filter-category`  | —             | Filter pages matching category substring                                  |
| `--filter-min-price` | `0`           | Minimum product price filter                                              |
| `--filter-max-price` | `0`           | Maximum product price filter                                              |
| `--filter-in-stock`  | `false`       | Filter only in-stock products                                             |
| `--filter-has-product` | `false`     | Filter only pages with structured product data                            |
| `--filter-keyword`   | —             | Filter pages matching keyword in title, body, or product                 |
| `-verbose`           | `false`       | Enable verbose / debug logging                                            |
| `-no-color`          | `false`       | Disable colored terminal output                                           |
| `--headless`         | `false`       | Enable headless browser rendering via ChromeDP                            |
| `--browser-visible`   | `false`       | Show browser window instead of headless (useful for debugging)            |
| `--browser-timeout`   | `30s`         | Browser page render and navigation timeout                                |
| `--browser-strategy`  | `always`      | Browser rendering strategy: `always` (direct ChromeDP) or `fallback` (Colly fast path, ChromeDP when JS is required) |
| `--click-crawl`       | `false`       | Enable click-based link discovery for interactive elements (buttons, SPA nav, load more) |
| `--click-fallback`    | `false`       | Shortcut: enables headless + click-crawl with fallback strategy           |
| `--max-clicks-per-page` | `10`        | Maximum interactive elements to click per page during click crawl         |
### Examples

**Basic crawl with defaults:**

```bash
spiderly -url https://example.com
```

**High-throughput parallel crawl:**

```bash
spiderly crawl https://example.com --max-pages 500 --chunked --chunk-size 100 --max-workers 8
```

**Crawl through a single proxy:**

```bash
spiderly crawl https://example.com --proxy http://127.0.0.1:8080
```

**Rotate proxies across chunk workers:**

```bash
SPIDERLY_PROXY=http://p1:8080,http://p2:8080 spiderly crawl https://example.com --chunked --max-workers 4

**News crawl with tag parsing:**

```bash
spiderly crawl https://news.example.com --news-mode --news-pattern "/news|article/" --news-sitemaps news,press,headline
```
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
│   │   └──        # Core configuration, main run loop, console UI
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

### Headless browser rendering & click crawler

Spiderly integrates [ChromeDP](https://github.com/chromedp/chromedp) to render client-side JavaScript applications (React, Next.js, Vue, Angular, SPAs) and discover links that only appear after clicking interactive elements.

#### Prerequisites
Google Chrome or Chromium must be installed on your system. On macOS and Linux, Spiderly automatically detects standard installation paths. If Chrome is not installed or fails to launch, Spiderly logs a clear warning and gracefully falls back to the Colly HTTP engine.

#### Browser Strategies & Fallback Policy
1. **Always (`--headless` or `--browser-strategy=always`)**:
   Every page is rendered directly through headless ChromeDP. Rendered DOM, HTTP status code, final redirected URL, and title are extracted and passed through the standard extraction pipeline.
2. **Fallback (`--browser-strategy=fallback` or `--click-fallback`)**:
   - **Step 1**: Fetch page using Colly (fast path).
   - **Step 2**: Inspect the response with `DetectJSRequired`: checks for `<noscript>` tags requiring JS, empty SPA root containers (`#root`, `#app`, `#__next`), Cloudflare/bot challenge pages, or minimal body text with scripts.
   - **Step 3**: If JS is required, automatically re-fetch the page using ChromeDP and replace the static shell with the rendered DOM.
   - **Step 4**: If `--click-crawl` is enabled, run interactive element discovery.

#### Click Crawler
When `--click-crawl` is enabled, Spiderly inspects the rendered DOM for interactive elements (`button`, `[role="button"]`, links without usable `href`, pagination, "Load more" triggers). It clicks up to `--max-clicks-per-page` (default: 10), intercepts URL changes or newly inserted `<a>` tags, and enqueues them into the crawl queue within the configured domain and depth limits.

#### Examples

```bash
# Render JS-heavy single-page applications with ChromeDP
spiderly crawl https://app.example.com --headless

# Use Colly fast path with ChromeDP fallback only when JS is detected
spiderly crawl https://example.com --headless --browser-strategy=fallback

# Crawl interactive pages and discover dynamically loaded links
spiderly crawl https://spa.example.com --headless --click-crawl --max-clicks-per-page 15

# Convenience shortcut for hybrid crawl + click discovery
spiderly crawl https://example.com --click-fallback

# Debug with a visible browser window
spiderly crawl https://example.com --headless --browser-visible --browser-timeout 45s
```

### Product mode & URL filters
When `-product-mode` is enabled (or a custom `-product-pattern` is provided), Spiderly auto-enables chunked processing, increases default page, chunk, and worker counts, and uses a regex to identify product pages. You can also supply a direct sitemap (`-sitemap`), filter by minimum priority (`-min-priority`), or apply a URL filter (`-url-pattern`), or force recursive link-following with `-recursive`. [`cmd/main.go:44-51`][`cmd/main.go:68-96`]

```bash
# Crawl products from sitemap with custom pattern and save JSON
spiderly -url https://example.com -product-mode -product-pattern "/product/" -output products.json
```

### News mode & tag parsing
When `--news-mode` is enabled (or a custom `--news-pattern` is provided), Spiderly prioritizes news-like sitemap files and parses article metadata from HTML/meta tags and page tag links. This mode does not rely on JSON-LD extraction.

```bash
spiderly crawl https://news.example.com --news-mode --news-pattern "/news|article/" --news-sitemaps news,press
```

### Chunked processing
Split large crawls into batches of `--chunk-size` URLs processed concurrently across `--max-workers`. Each chunk worker still respects `--concurrency`, `--delay`, and `--timeout` to maintain throughput and politeness.

```bash
# Process 1000 pages in chunks of 200 across 5 workers
spiderly crawl https://example.com --max-pages 1000 --chunked --chunk-size 200 --max-workers 5
```

### Proxy configuration
Use `--proxy` for explicit proxy URLs, or `SPIDERLY_PROXY` for environment-based configuration.

```bash
# Single proxy
spiderly crawl https://example.com --proxy http://127.0.0.1:8080

# Multiple proxies (comma-separated)
SPIDERLY_PROXY=http://p1:8080,http://p2:8080 spiderly crawl https://example.com --chunked --max-workers 4
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
