package core

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
)

// ─────────────────────────────────────────────
//  Tracking / Junk Query Parameters
// ─────────────────────────────────────────────

// trackingParams are query parameters injected by analytics platforms
// and ad networks.  They never change page content so we strip them
// during normalisation to avoid crawling the same page twice.
var trackingParams = map[string]bool{
	// Google Analytics / Ads
	"utm_source":   true,
	"utm_medium":   true,
	"utm_campaign": true,
	"utm_term":     true,
	"utm_content":  true,
	"utm_id":       true,
	"gclid":        true,
	"gclsrc":       true,
	"dclid":        true,
	"gbraid":       true,
	"wbraid":       true,

	// Facebook / Meta
	"fbclid": true,
	"fb_action_ids":  true,
	"fb_action_types": true,
	"fb_source":       true,
	"fb_ref":          true,

	// Microsoft / Bing
	"msclkid": true,

	// HubSpot
	"hsa_cam": true,
	"hsa_grp": true,
	"hsa_mt":  true,
	"hsa_src": true,
	"hsa_ad":  true,
	"hsa_acc": true,
	"hsa_net": true,
	"hsa_ver": true,
	"hsa_la":  true,
	"hsa_ol":  true,
	"hsa_kw":  true,
	"hsa_tgt": true,

	// Mailchimp
	"mc_cid": true,
	"mc_eid": true,

	// Generic tracking / session
	"ref":          true,
	"source":       true,
	"_ga":          true,
	"_gl":          true,
	"_hsenc":       true,
	"_hsmi":        true,
	"_openstat":    true,
	"yclid":        true,
	"wickedid":     true,
	"twclid":       true,
	"ttclid":       true,
	"li_fat_id":    true,
	"igshid":       true,
	"s_kwcid":      true,
	"ef_id":        true,
	"srsltid":      true,

	// Session / cache-busting
	"sessionid":  true,
	"session_id": true,
	"sid":        true,
	"PHPSESSID":  true,
	"jsessionid": true,
	"_t":         true,
	"_ts":        true,
	"timestamp":  true,
	"cb":         true,
	"nocache":    true,
	"rand":       true,
}

// ─────────────────────────────────────────────
//  Default Ports (stripped during normalisation)
// ─────────────────────────────────────────────

var defaultPorts = map[string]string{
	"http":  "80",
	"https": "443",
	"ftp":   "21",
}

// ─────────────────────────────────────────────
//  Non-Page Extensions (skip these entirely)
// ─────────────────────────────────────────────

var nonPageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".svg": true, ".webp": true, ".ico": true, ".bmp": true,
	".tiff": true, ".avif": true,

	".pdf": true, ".doc": true, ".docx": true, ".xls": true,
	".xlsx": true, ".ppt": true, ".pptx": true, ".odt": true,

	".zip": true, ".tar": true, ".gz": true, ".rar": true,
	".7z": true, ".bz2": true,

	".mp3": true, ".mp4": true, ".avi": true, ".mov": true,
	".wmv": true, ".flv": true, ".webm": true, ".ogg": true,
	".wav": true, ".flac": true,

	".css": true, ".js": true, ".map": true, ".woff": true,
	".woff2": true, ".ttf": true, ".eot": true,

	".xml": true, ".json": true, ".rss": true, ".atom": true,
	".txt": true, ".csv": true,

	".exe": true, ".dmg": true, ".apk": true, ".msi": true,
}


// ─────────────────────────────────────────────
//  URLNormalizer — Normalize + Dedup facade
// ─────────────────────────────────────────────

// URLNormalizer combines URL canonicalization with seen-tracking.
// This is the type referenced by Core.normalizer.
type URLNormalizer struct {
	dedup *Deduplicator
}

// NewURLNormalizer creates a URLNormalizer with a default-sized deduplicator.
func NewURLNormalizer() *URLNormalizer {
	return &URLNormalizer{
		dedup: NewDeduplicator(0),
	}
}

// Normalize canonicalizes a raw URL using the package-level NormalizeURL.
func (n *URLNormalizer) Normalize(rawURL string) (string, error) {
	result := NormalizeURL(rawURL)
	if result == "" {
		return "", NewValidationError("empty URL after normalization")
	}
	return result, nil
}

// IsSeen reports whether the normalized URL has already been tracked.
func (n *URLNormalizer) IsSeen(normalizedURL string) bool {
	return n.dedup.Seen(normalizedURL)
}

// MarkSeen records a normalized URL as visited.
func (n *URLNormalizer) MarkSeen(normalizedURL string) {
	n.dedup.SeenOrAdd(normalizedURL)
}

// Count returns the number of unique URLs tracked so far.
func (n *URLNormalizer) Count() int {
	return n.dedup.Count()
}

// Reset clears all tracking state.
func (n *URLNormalizer) Reset() {
	n.dedup.Reset()
}

// ─────────────────────────────────────────────
//  NormalizeURL
// ─────────────────────────────────────────────

// NormalizeURL canonicalises a raw URL so that trivially-different
// strings that point to the same resource produce the same output.
//
// Transformations applied:
//   - Lowercase scheme and host
//   - Remove default port (:80 for http, :443 for https)
//   - Decode unnecessary percent-encoding in the path
//   - Collapse // and resolve . / .. in the path
//   - Remove trailing slash (except bare root "/")
//   - Remove fragment (#section)
//   - Remove known tracking / session query parameters
//   - Sort remaining query parameters alphabetically
//   - Remove empty query string
func NormalizeURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL // return as-is if unparseable
	}

	// ── Scheme ──
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme == "" {
		parsed.Scheme = "https"
	}

	// ── Host ──
	parsed.Host = strings.ToLower(parsed.Host)

	// Strip default port
	if host, port, hasPort := splitHostPort(parsed.Host); hasPort {
		if dp, ok := defaultPorts[parsed.Scheme]; ok && port == dp {
			parsed.Host = host
		}
	}

	// Remove trailing dot in hostname (DNS root)
	parsed.Host = strings.TrimRight(parsed.Host, ".")

	// Remove www. prefix for dedup (optional — common convention)
	// Disabled by default because www and non-www can serve different content.
	// Uncomment if your use-case treats them as identical:
	// parsed.Host = strings.TrimPrefix(parsed.Host, "www.")

	// ── Path ──
	// Clean resolves . and .. and collapses double slashes
	if parsed.Path == "" {
		parsed.Path = "/"
	} else {
		parsed.Path = path.Clean(parsed.Path)
		// path.Clean strips trailing slash; keep root
		if parsed.Path == "." {
			parsed.Path = "/"
		}
	}

	// Decode unreserved percent-encoded characters (%41 → A, etc.)
	parsed.Path = decodeUnreserved(parsed.Path)

	// Remove trailing slash (except root)
	if len(parsed.Path) > 1 {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}

	// ── Fragment ──
	parsed.Fragment = ""
	parsed.RawFragment = ""

	// ── Query ──
	parsed.RawQuery = normalizeQuery(parsed.Query())

	// ── User info ── (strip credentials from URLs)
	parsed.User = nil

	return parsed.String()
}

// ─────────────────────────────────────────────
//  Query Normalisation
// ─────────────────────────────────────────────

// normalizeQuery removes tracking params, sorts the rest, and returns
// a canonical query string.  Returns "" if nothing remains.
func normalizeQuery(values url.Values) string {
	// Remove tracking / junk parameters
	for param := range values {
		lower := strings.ToLower(param)
		if trackingParams[lower] {
			values.Del(param)
			continue
		}
		// Also remove params whose value is empty and name looks like a hash
		if len(param) > 20 && values.Get(param) == "" {
			values.Del(param)
		}
	}

	if len(values) == 0 {
		return ""
	}

	// Collect and sort keys
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build sorted query string
	var buf strings.Builder
	first := true
	for _, k := range keys {
		vals := values[k]
		sort.Strings(vals) // deterministic multi-value order
		for _, v := range vals {
			if !first {
				buf.WriteByte('&')
			}
			first = false
			buf.WriteString(url.QueryEscape(k))
			if v != "" {
				buf.WriteByte('=')
				buf.WriteString(url.QueryEscape(v))
			}
		}
	}
	return buf.String()
}

// ─────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────

// splitHostPort splits "host:port" and reports whether a port was present.
func splitHostPort(hostport string) (host, port string, hasPort bool) {
	// Handle IPv6 [::1]:8080
	if idx := strings.LastIndex(hostport, ":"); idx != -1 {
		// Make sure the colon isn't inside brackets
		if bracketIdx := strings.LastIndex(hostport, "]"); bracketIdx < idx {
			return hostport[:idx], hostport[idx+1:], true
		}
	}
	return hostport, "", false
}

// decodeUnreserved decodes percent-encoded characters that RFC 3986
// defines as "unreserved" (ALPHA, DIGIT, - . _ ~).  Everything else
// stays encoded.
func decodeUnreserved(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))

	i := 0
	for i < len(s) {
		if s[i] == '%' && i+2 < len(s) {
			hi := unhex(s[i+1])
			lo := unhex(s[i+2])
			if hi >= 0 && lo >= 0 {
				ch := byte(hi<<4 | lo)
				if isUnreserved(ch) {
					buf.WriteByte(ch)
					i += 3
					continue
				}
				// Keep encoded but uppercase the hex digits
				buf.WriteByte('%')
				buf.WriteByte(upperHex(s[i+1]))
				buf.WriteByte(upperHex(s[i+2]))
				i += 3
				continue
			}
		}
		buf.WriteByte(s[i])
		i++
	}
	return buf.String()
}

func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '-' || c == '.' || c == '_' || c == '~'
}

func unhex(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

func upperHex(c byte) byte {
	if c >= 'a' && c <= 'f' {
		return c - 32
	}
	return c
}

// IsPageURL returns false for URLs pointing to images, PDFs, fonts,
// and other non-HTML resources based on file extension.
func IsPageURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	if ext == "" {
		return true // no extension → assume HTML
	}
	return !nonPageExtensions[ext]
}

// URLFingerprint returns a short hex digest of the normalised URL.
// Useful as a map key or database identifier.
func URLFingerprint(rawURL string) string {
	normalized := NormalizeURL(rawURL)
	hash := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", hash[:12]) // 24 hex chars = 96 bits
}

// SameDomain reports whether two URLs share the same registered domain.
func SameDomain(a, b string) bool {
	pa, err1 := url.Parse(NormalizeURL(a))
	pb, err2 := url.Parse(NormalizeURL(b))
	if err1 != nil || err2 != nil {
		return false
	}
	return extractDomain(pa.Host) == extractDomain(pb.Host)
}

// extractDomain returns the last two labels of a hostname.
// "shop.cdn.example.com" → "example.com"
func extractDomain(host string) string {
	host = strings.TrimSuffix(host, ".")
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// ─────────────────────────────────────────────
//  Deduplicator
// ─────────────────────────────────────────────

// Deduplicator tracks which URLs have already been seen.
// All URLs are normalised before checking so trivially-different
// variants of the same page are caught.
//
// It uses a two-tier approach:
//   - A fast Bloom-filter-style bitmap for the common "not seen" path
//   - An exact hash set to handle the rare false-positive case
//
// For crawls under ~500k URLs the bitmap tier is skipped and only
// the exact set is used (the overhead is negligible at that scale).
type Deduplicator struct {
	mu   sync.RWMutex
	seen map[string]bool // normalised URL → true

	// Bloom filter tier (initialised lazily for large crawls)
	bitmap    []uint64
	bitmapLen uint64
	useBloom  bool

	stats DeduplicatorStats
}

// DeduplicatorStats exposes hit/miss counters.
type DeduplicatorStats struct {
	Checked    int64
	Duplicates int64
	Unique     int64
}

// NewDeduplicator creates a deduplicator.
// expectedSize hints at the number of URLs you expect to process;
// pass 0 to use the exact-set-only mode.
func NewDeduplicator(expectedSize int) *Deduplicator {
	d := &Deduplicator{
		seen: make(map[string]bool, expectedSize),
	}

	// Enable bloom tier for large crawls (>10k expected URLs)
	if expectedSize > 10_000 {
		// Size the bitmap at ~10 bits per element for ~1% FP rate
		bits := uint64(expectedSize) * 10
		words := (bits + 63) / 64
		d.bitmap = make([]uint64, words)
		d.bitmapLen = words * 64
		d.useBloom = true
	}

	return d
}

// SeenOrAdd normalises the URL, checks whether it has been seen
// before, and if not marks it as seen.  Returns true if the URL
// was already known (i.e. is a duplicate).
func (d *Deduplicator) SeenOrAdd(rawURL string) bool {
	normalized := NormalizeURL(rawURL)
	if normalized == "" {
		return true // treat empty as "already seen"
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.stats.Checked++

	// ── Fast path: bloom check ──
	if d.useBloom {
		h1, h2 := d.hashes(normalized)
		if !d.bloomTest(h1, h2) {
			// Definitely not seen → add to both tiers
			d.bloomSet(h1, h2)
			d.seen[normalized] = true
			d.stats.Unique++
			return false
		}
		// Bloom says "maybe" → fall through to exact check
	}

	// ── Exact check ──
	if d.seen[normalized] {
		d.stats.Duplicates++
		return true
	}

	// Not a duplicate
	d.seen[normalized] = true
	if d.useBloom {
		h1, h2 := d.hashes(normalized)
		d.bloomSet(h1, h2)
	}
	d.stats.Unique++
	return false
}

// Seen checks without adding.
func (d *Deduplicator) Seen(rawURL string) bool {
	normalized := NormalizeURL(rawURL)
	if normalized == "" {
		return true
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	// Quick bloom reject
	if d.useBloom {
		h1, h2 := d.hashes(normalized)
		if !d.bloomTest(h1, h2) {
			return false
		}
	}

	return d.seen[normalized]
}

// Count returns the number of unique URLs tracked.
func (d *Deduplicator) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.seen)
}

// Stats returns a snapshot of dedup statistics.
func (d *Deduplicator) Stats() DeduplicatorStats {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.stats
}

// Reset clears all state.
func (d *Deduplicator) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.seen = make(map[string]bool, len(d.seen))
	d.stats = DeduplicatorStats{}

	if d.useBloom {
		for i := range d.bitmap {
			d.bitmap[i] = 0
		}
	}
}

// ─────────────────────────────────────────────
//  Bloom Filter Internals
// ─────────────────────────────────────────────

// hashes produces two independent hash values using FNV-inspired mixing.
// We use double-hashing to derive k probe positions: h(i) = h1 + i*h2.
func (d *Deduplicator) hashes(s string) (uint64, uint64) {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)

	h1 := uint64(offset64)
	h2 := h1 * 3 // runtime multiplication — wraps as uint64

	for i := 0; i < len(s); i++ {
		h1 ^= uint64(s[i])
		h1 *= prime64

		h2 ^= uint64(s[len(s)-1-i])
		h2 *= prime64
	}

	return h1, h2
}


// bloomTest checks k=4 probe positions.
func (d *Deduplicator) bloomTest(h1, h2 uint64) bool {
	for i := uint64(0); i < 4; i++ {
		pos := (h1 + i*h2) % d.bitmapLen
		word := pos / 64
		bit := pos % 64
		if d.bitmap[word]&(1<<bit) == 0 {
			return false
		}
	}
	return true
}

// bloomSet sets k=4 probe positions.
func (d *Deduplicator) bloomSet(h1, h2 uint64) {
	for i := uint64(0); i < 4; i++ {
		pos := (h1 + i*h2) % d.bitmapLen
		word := pos / 64
		bit := pos % 64
		d.bitmap[word] |= 1 << bit
	}
}