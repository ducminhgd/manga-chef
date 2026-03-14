package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/ducminhgd/manga-chef/pkg/sources"
	"gopkg.in/yaml.v3"
)

// codePattern defines the allowed format for source codes:
// one or more lowercase letters, digits, or underscores.
var codePattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// apiScrapers lists built-in scrapers that use a structured API rather than
// HTML scraping. These are exempt from the rate_limit_ms warning because API
// servers are designed to handle concurrent requests.
var apiScrapers = map[string]bool{
	"mangadex": true,
}

// Validator checks a slice of SourceConfig values against the rules defined
// in the project plan (T-15.x).
//
// It is intentionally decoupled from the scraper registry: the scraper package
// depends on config (to read SourceConfig), so config must not import scraper.
// The caller may populate KnownScrapers after the registry is initialised.
type Validator struct {
	// KnownScrapers is the list of registered built-in scraper names.
	// When nil or empty, the validator skips built-in name checking; names are
	// validated at runtime when the scraper registry is queried.
	// Plugin file paths (anything containing a path separator or dot-extension)
	// are always validated regardless of this field.
	KnownScrapers []string
}

// sourceNodeInfo holds the line numbers of key fields within a single source
// entry, extracted from the raw yaml.Node tree before struct decoding.
// A line of 0 means the field was absent or could not be located.
type sourceNodeInfo struct {
	startLine   int // line of the "- " sequence item
	nameLine    int
	codeLine    int
	baseURLLine int
	scraperLine int
}

// Validate checks cfgs against all validation rules and returns the complete
// list of issues found. It does not stop at the first error — all sources are
// checked so that the user can fix everything in one pass.
//
// nodeInfos provides line-number context for error messages; it is aligned
// with cfgs (index 0 → cfgs[0]).
func (v *Validator) Validate(cfgs []sources.SourceConfig, nodeInfos []sourceNodeInfo, file string) ValidationErrors {
	var errs ValidationErrors
	seenCodes := make(map[string]int) // code → first index with that code

	for i, cfg := range cfgs {
		ni := sourceNodeInfo{}
		if i < len(nodeInfos) {
			ni = nodeInfos[i]
		}
		prefix := fmt.Sprintf("sources[%d]", i)

		// ── name ─────────────────────────────────────────────────────────
		if strings.TrimSpace(cfg.Name) == "" {
			errs = append(errs, ValidationError{
				Severity: SeverityError,
				File:     file,
				Line:     ni.nameLine,
				Field:    prefix + ".name",
				Message:  "name is required",
			})
		}

		// ── T-15.1: code ─────────────────────────────────────────────────
		switch {
		case strings.TrimSpace(cfg.Code) == "":
			errs = append(errs, ValidationError{
				Severity: SeverityError,
				File:     file,
				Line:     ni.codeLine,
				Field:    prefix + ".code",
				Message:  "code is required",
			})

		case !codePattern.MatchString(cfg.Code):
			errs = append(errs, ValidationError{
				Severity: SeverityError,
				File:     file,
				Line:     ni.codeLine,
				Field:    prefix + ".code",
				Message:  fmt.Sprintf("code %q must contain only lowercase letters, digits, and underscores (no spaces, hyphens, or uppercase)", cfg.Code),
			})

		default:
			// T-15.1: check for within-file duplicates.
			if firstIdx, dup := seenCodes[cfg.Code]; dup {
				errs = append(errs, ValidationError{
					Severity: SeverityError,
					File:     file,
					Line:     ni.codeLine,
					Field:    prefix + ".code",
					Message:  fmt.Sprintf("duplicate code %q — first defined at sources[%d]", cfg.Code, firstIdx),
				})
			} else {
				seenCodes[cfg.Code] = i
			}
		}

		// ── T-15.2: base_url ─────────────────────────────────────────────
		if strings.TrimSpace(cfg.BaseURL) == "" {
			errs = append(errs, ValidationError{
				Severity: SeverityError,
				File:     file,
				Line:     ni.baseURLLine,
				Field:    prefix + ".base_url",
				Message:  "base_url is required",
			})
		} else if err := validateURL(cfg.BaseURL); err != nil {
			errs = append(errs, ValidationError{
				Severity: SeverityError,
				File:     file,
				Line:     ni.baseURLLine,
				Field:    prefix + ".base_url",
				Message:  fmt.Sprintf("invalid URL: %s", err),
			})
		}

		// ── T-15.3: scraper ──────────────────────────────────────────────
		if strings.TrimSpace(cfg.Scraper) == "" {
			errs = append(errs, ValidationError{
				Severity: SeverityError,
				File:     file,
				Line:     ni.scraperLine,
				Field:    prefix + ".scraper",
				Message:  "scraper is required — use a built-in name (e.g. \"generic\") or a plugin file path",
			})
		} else if isFilePath(cfg.Scraper) {
			// File path: must exist on disk.
			if _, err := os.Stat(cfg.Scraper); os.IsNotExist(err) {
				errs = append(errs, ValidationError{
					Severity: SeverityError,
					File:     file,
					Line:     ni.scraperLine,
					Field:    prefix + ".scraper",
					Message:  fmt.Sprintf("scraper plugin file %q does not exist", cfg.Scraper),
				})
			}
		} else if len(v.KnownScrapers) > 0 && !v.isKnownScraper(cfg.Scraper) {
			// Built-in name: validate only when registry is populated.
			errs = append(errs, ValidationError{
				Severity: SeverityError,
				File:     file,
				Line:     ni.scraperLine,
				Field:    prefix + ".scraper",
				Message:  fmt.Sprintf("unknown scraper %q — known scrapers: %s", cfg.Scraper, strings.Join(v.KnownScrapers, ", ")),
			})
		}

		// ── T-15.4: rate_limit_ms warning for HTML sources ───────────────
		if cfg.RateLimitMs == 0 && !apiScrapers[cfg.Scraper] && !isFilePath(cfg.Scraper) {
			errs = append(errs, ValidationError{
				Severity: SeverityWarning,
				File:     file,
				Line:     ni.scraperLine,
				Field:    prefix + ".rate_limit_ms",
				Message:  "rate_limit_ms is not set — consider adding a delay (e.g. 500ms) to avoid overloading HTML sources",
			})
		}

		// ── selectors required for generic scraper ───────────────────────
		if cfg.Scraper == "generic" && cfg.Selectors == nil {
			errs = append(errs, ValidationError{
				Severity: SeverityError,
				File:     file,
				Line:     ni.scraperLine,
				Field:    prefix + ".selectors",
				Message:  `scraper "generic" requires a selectors block`,
			})
		}
	}

	return errs
}

// validateURL returns an error if s is not an absolute HTTP or HTTPS URL.
func validateURL(s string) error {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("missing host")
	}
	return nil
}

// isFilePath returns true when s looks like a filesystem path rather than a
// built-in scraper name. We detect paths by:
//   - relative paths starting with ./ or ../
//   - absolute paths starting with /
//   - any string containing the OS path separator
func isFilePath(s string) bool {
	return strings.HasPrefix(s, "./") ||
		strings.HasPrefix(s, "../") ||
		strings.HasPrefix(s, "/") ||
		strings.ContainsRune(s, os.PathSeparator)
}

// isKnownScraper returns true if name matches one of the registered scrapers.
func (v *Validator) isKnownScraper(name string) bool {
	for _, k := range v.KnownScrapers {
		if k == name {
			return true
		}
	}
	return false
}

// ── yaml.Node helpers ────────────────────────────────────────────────────────

// extractNodeInfos builds a []sourceNodeInfo aligned with the sources sequence
// in the YAML node tree. Index i corresponds to the i-th source entry.
func extractNodeInfos(root *yaml.Node) []sourceNodeInfo {
	nodes := sourceMappingNodes(root)
	infos := make([]sourceNodeInfo, len(nodes))
	for i, n := range nodes {
		infos[i] = nodeInfo(n)
	}
	return infos
}

// sourceMappingNodes returns the yaml.Node items inside the top-level
// "sources" sequence node.
func sourceMappingNodes(root *yaml.Node) []*yaml.Node {
	// Unwrap DocumentNode.
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil
	}
	// MappingNode.Content alternates: key₀, value₀, key₁, value₁, …
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "sources" {
			seq := root.Content[i+1]
			if seq.Kind == yaml.SequenceNode {
				return seq.Content
			}
		}
	}
	return nil
}

// nodeInfo extracts the line numbers of named fields from a single source
// mapping node. Missing fields are left as 0.
func nodeInfo(n *yaml.Node) sourceNodeInfo {
	info := sourceNodeInfo{startLine: n.Line}
	if n.Kind != yaml.MappingNode {
		return info
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i]
		switch key.Value {
		case "name":
			info.nameLine = key.Line
		case "code":
			info.codeLine = key.Line
		case "base_url":
			info.baseURLLine = key.Line
		case "scraper":
			info.scraperLine = key.Line
		}
	}
	return info
}
