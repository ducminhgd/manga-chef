package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"github.com/ducminhgd/manga-chef/pkg/sources"
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

		v.validateName(&errs, cfg.Name, file, ni.nameLine, prefix)
		v.validateCode(&errs, cfg.Code, file, ni.codeLine, prefix, i, seenCodes)
		v.validateBaseURL(&errs, cfg.BaseURL, file, ni.baseURLLine, prefix)
		v.validateScraper(&errs, cfg.Scraper, file, ni.scraperLine, prefix)
		v.validateRateLimit(&errs, cfg.RateLimitMs, cfg.Scraper, file, ni.scraperLine, prefix)
		v.validateSelectors(&errs, cfg.Scraper, cfg.Selectors, file, ni.scraperLine, prefix)
	}

	return errs
}

func (v *Validator) validateName(errs *ValidationErrors, name, file string, line int, prefix string) {
	if strings.TrimSpace(name) == "" {
		*errs = append(*errs, ValidationError{
			Severity: SeverityError,
			File:     file,
			Line:     line,
			Field:    prefix + ".name",
			Message:  "name is required",
		})
	}
}

func (v *Validator) validateCode(errs *ValidationErrors, code, file string, line int, prefix string, idx int, seenCodes map[string]int) {
	switch {
	case strings.TrimSpace(code) == "":
		*errs = append(*errs, ValidationError{
			Severity: SeverityError,
			File:     file,
			Line:     line,
			Field:    prefix + ".code",
			Message:  "code is required",
		})
	case !codePattern.MatchString(code):
		*errs = append(*errs, ValidationError{
			Severity: SeverityError,
			File:     file,
			Line:     line,
			Field:    prefix + ".code",
			Message:  fmt.Sprintf("code %q must contain only lowercase letters, digits, and underscores (no spaces, hyphens, or uppercase)", code),
		})
	default:
		if firstIdx, dup := seenCodes[code]; dup {
			*errs = append(*errs, ValidationError{
				Severity: SeverityError,
				File:     file,
				Line:     line,
				Field:    prefix + ".code",
				Message:  fmt.Sprintf("duplicate code %q — first defined at sources[%d]", code, firstIdx),
			})
		} else {
			seenCodes[code] = idx
		}
	}
}

func (v *Validator) validateBaseURL(errs *ValidationErrors, baseURL, file string, line int, prefix string) {
	if strings.TrimSpace(baseURL) == "" {
		*errs = append(*errs, ValidationError{
			Severity: SeverityError,
			File:     file,
			Line:     line,
			Field:    prefix + ".base_url",
			Message:  "base_url is required",
		})
	} else if err := validateURL(baseURL); err != nil {
		*errs = append(*errs, ValidationError{
			Severity: SeverityError,
			File:     file,
			Line:     line,
			Field:    prefix + ".base_url",
			Message:  fmt.Sprintf("invalid URL: %s", err),
		})
	}
}

func (v *Validator) validateScraper(errs *ValidationErrors, scraper, file string, line int, prefix string) {
	if strings.TrimSpace(scraper) == "" {
		*errs = append(*errs, ValidationError{
			Severity: SeverityError,
			File:     file,
			Line:     line,
			Field:    prefix + ".scraper",
			Message:  "scraper is required — use a built-in name (e.g. \"generic\") or a plugin file path",
		})
		return
	}

	if isFilePath(scraper) {
		if _, err := os.Stat(scraper); os.IsNotExist(err) {
			*errs = append(*errs, ValidationError{
				Severity: SeverityError,
				File:     file,
				Line:     line,
				Field:    prefix + ".scraper",
				Message:  fmt.Sprintf("scraper plugin file %q does not exist", scraper),
			})
		}
		return
	}

	if len(v.KnownScrapers) > 0 && !v.isKnownScraper(scraper) {
		*errs = append(*errs, ValidationError{
			Severity: SeverityError,
			File:     file,
			Line:     line,
			Field:    prefix + ".scraper",
			Message:  fmt.Sprintf("unknown scraper %q — known scrapers: %s", scraper, strings.Join(v.KnownScrapers, ", ")),
		})
	}
}

func (v *Validator) validateRateLimit(errs *ValidationErrors, rateLimit int, scraper, file string, line int, prefix string) {
	if rateLimit == 0 && !apiScrapers[scraper] && !isFilePath(scraper) {
		*errs = append(*errs, ValidationError{
			Severity: SeverityWarning,
			File:     file,
			Line:     line,
			Field:    prefix + ".rate_limit_ms",
			Message:  "rate_limit_ms is not set — consider adding a delay (e.g. 500ms) to avoid overloading HTML sources",
		})
	}
}

func (v *Validator) validateSelectors(errs *ValidationErrors, scraper string, selectors *sources.Selectors, file string, line int, prefix string) {
	if scraper == "generic" && selectors == nil {
		*errs = append(*errs, ValidationError{
			Severity: SeverityError,
			File:     file,
			Line:     line,
			Field:    prefix + ".selectors",
			Message:  `scraper "generic" requires a selectors block`,
		})
	}
}

// validateURL returns an error if s is not an absolute HTTP or HTTPS URL.
func validateURL(s string) error {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return fmt.Errorf("parsing URL: %w", err)
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

// ── yaml.Node helpers ────────────────────────────────────────────────────────.

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
