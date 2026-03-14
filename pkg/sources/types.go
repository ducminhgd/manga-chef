// Package sources defines the public domain types shared across all packages
// in Manga Chef. No business logic lives here — only data shapes.
//
// Keeping types in a separate package prevents import cycles: internal packages
// (config, scraper, downloader) all depend on sources, but sources depends on
// nothing inside the project.
package sources

// SourceConfig represents a single manga source declared in a YAML sources file.
// It is the public contract between the config loader, the scraper registry,
// and the downloader.
type SourceConfig struct {
	// Name is the human-readable display name shown in the CLI (e.g. "MangaDex").
	Name string `yaml:"name"`

	// Code is the short, unique identifier used with --source on the CLI
	// (e.g. "mangadex"). Must be lowercase alphanumeric with underscores only.
	Code string `yaml:"code"`

	// BaseURL is the root URL of the site. This is the only field that changes
	// when switching between a source and its mirrors.
	BaseURL string `yaml:"base_url"`

	// Scraper is either a built-in scraper name (e.g. "mangadex", "generic")
	// or a relative file path to a custom plugin (e.g. "./scrapers/custom.py").
	Scraper string `yaml:"scraper"`

	// Enabled controls whether this source is active. Defaults to true if omitted.
	Enabled *bool `yaml:"enabled,omitempty"`

	// Headers are extra HTTP headers sent with every request to this source.
	// Common use: Referer, User-Agent for CDN image servers.
	Headers map[string]string `yaml:"headers,omitempty"`

	// RateLimitMs is the minimum delay in milliseconds between sequential requests.
	// Set to 0 to disable rate limiting (not recommended for HTML sources).
	RateLimitMs int `yaml:"rate_limit_ms,omitempty"`

	// Retries is the maximum number of retry attempts for a failed image download.
	// Defaults to DefaultRetries if not set.
	Retries int `yaml:"retries,omitempty"`

	// TimeoutS is the HTTP request timeout in seconds.
	// Defaults to DefaultTimeoutS if not set.
	TimeoutS int `yaml:"timeout_s,omitempty"`

	// Selectors holds CSS selector strings used by the "generic" scraper.
	// Required when Scraper is "generic"; ignored for all other scraper types.
	Selectors *Selectors `yaml:"selectors,omitempty"`
}

// Default configuration values applied when the corresponding fields are unset.
const (
	DefaultRetries  = 3
	DefaultTimeoutS = 30
)

// IsEnabled returns whether the source is active.
// Defaults to true when the Enabled field is absent from YAML.
func (s SourceConfig) IsEnabled() bool {
	if s.Enabled == nil {
		return true
	}
	return *s.Enabled
}

// EffectiveRetries returns the configured retry count, falling back to
// DefaultRetries when the field is zero or negative.
func (s SourceConfig) EffectiveRetries() int {
	if s.Retries <= 0 {
		return DefaultRetries
	}
	return s.Retries
}

// EffectiveTimeoutS returns the configured request timeout in seconds,
// falling back to DefaultTimeoutS when the field is zero or negative.
func (s SourceConfig) EffectiveTimeoutS() int {
	if s.TimeoutS <= 0 {
		return DefaultTimeoutS
	}
	return s.TimeoutS
}

// Selectors holds CSS selector expressions for the generic HTML scraper.
//
// Each field targets a specific part of the page. Values may use the "attr:"
// prefix to extract an element attribute instead of text content:
//
//	image_url: "attr:data-src"   # reads the data-src attribute
//	image_url: "attr:src"        # reads the src attribute
type Selectors struct {
	// ChapterList selects the anchor elements in the manga's chapter list.
	// Example: "ul.list-chapter li a"
	ChapterList string `yaml:"chapter_list"`

	// ChapterNumber extracts the chapter number from within a chapter list item.
	// Example: "attr:data-chapter"
	ChapterNumber string `yaml:"chapter_number"`

	// ImageList selects all image elements on a chapter reader page.
	// Example: "div.page-chapter img"
	ImageList string `yaml:"image_list"`

	// ImageURL extracts the image URL from each image element.
	// Use "attr:data-src" for lazy-loaded images, "attr:src" otherwise.
	ImageURL string `yaml:"image_url"`
}

// Chapter represents a single chapter returned by a scraper's chapter-list call.
type Chapter struct {
	// Number is the chapter number. Float64 supports fractional chapters (e.g. 10.5).
	Number float64

	// Title is the chapter title as shown on the source site.
	Title string

	// URL is the absolute URL to the chapter reader page.
	URL string
}

// Page represents a single image page within a downloaded chapter.
type Page struct {
	// Number is the 1-based page index within the chapter.
	Number int

	// URL is the absolute URL of the page image.
	URL string
}
