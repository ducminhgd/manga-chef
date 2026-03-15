package config_test

import (
	"os"
	"testing"

	"github.com/ducminhgd/manga-chef/internal/config"
	"github.com/ducminhgd/manga-chef/pkg/sources"
)

// validSource returns a fully valid SourceConfig to use as a base for tests
// that only need to change one field.
func validSource() sources.SourceConfig {
	return sources.SourceConfig{
		Name:        "TruyenQQ",
		Code:        "truyenqq",
		BaseURL:     "https://truyenqqno.com",
		Scraper:     "truyenqq",
		RateLimitMs: 500,
	}
}

func validate(cfgs []sources.SourceConfig, knownScrapers ...string) config.ValidationErrors {
	v := &config.Validator{KnownScrapers: knownScrapers}
	return v.Validate(cfgs, nil, "test.yml")
}

// ── name ─────────────────────────────────────────────────────────────────────

func TestValidator_NameRequired(t *testing.T) {
	cfg := validSource()
	cfg.Name = ""
	errs := validate([]sources.SourceConfig{cfg})
	assertHasErrorField(t, errs, "sources[0].name")
}

func TestValidator_NameWhitespaceIsEmpty(t *testing.T) {
	cfg := validSource()
	cfg.Name = "   "
	errs := validate([]sources.SourceConfig{cfg})
	assertHasErrorField(t, errs, "sources[0].name")
}

// ── code (T-15.1) ────────────────────────────────────────────────────────────

func TestValidator_CodeRequired(t *testing.T) {
	cfg := validSource()
	cfg.Code = ""
	errs := validate([]sources.SourceConfig{cfg})
	assertHasErrorField(t, errs, "sources[0].code")
}

func TestValidator_CodeValid(t *testing.T) {
	cases := []string{"abc", "abc123", "abc_123", "a", "a_b_c_1"}
	for _, code := range cases {
		t.Run(code, func(t *testing.T) {
			cfg := validSource()
			cfg.Code = code
			errs := validate([]sources.SourceConfig{cfg})
			assertNoErrorField(t, errs, "sources[0].code")
		})
	}
}

func TestValidator_CodeInvalidChars(t *testing.T) {
	cases := []struct {
		code string
		desc string
	}{
		{"Truyen", "uppercase"},
		{"truyen-qq", "hyphen"},
		{"truyen qq", "space"},
		{"truyen.qq", "dot"},
		{"MANGADEX", "all uppercase"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := validSource()
			cfg.Code = tc.code
			errs := validate([]sources.SourceConfig{cfg})
			assertHasErrorField(t, errs, "sources[0].code")
		})
	}
}

func TestValidator_DuplicateCodeWithinFile(t *testing.T) {
	a := validSource()
	b := validSource()
	b.Name = "TruyenQQ Mirror"
	b.BaseURL = "https://mirror.com"
	errs := validate([]sources.SourceConfig{a, b})
	assertHasErrorField(t, errs, "sources[1].code")
}

func TestValidator_UniqueCodes(t *testing.T) {
	a := validSource()
	b := validSource()
	b.Code = "truyenqq_mirror"
	b.Name = "TruyenQQ Mirror"
	errs := validate([]sources.SourceConfig{a, b})
	assertNoErrorField(t, errs, "sources[0].code")
	assertNoErrorField(t, errs, "sources[1].code")
}

// ── base_url (T-15.2) ────────────────────────────────────────────────────────

func TestValidator_BaseURLRequired(t *testing.T) {
	cfg := validSource()
	cfg.BaseURL = ""
	errs := validate([]sources.SourceConfig{cfg})
	assertHasErrorField(t, errs, "sources[0].base_url")
}

func TestValidator_BaseURLValidHTTPS(t *testing.T) {
	cfg := validSource()
	cfg.BaseURL = "https://example.com"
	errs := validate([]sources.SourceConfig{cfg})
	assertNoErrorField(t, errs, "sources[0].base_url")
}

func TestValidator_BaseURLValidHTTP(t *testing.T) {
	cfg := validSource()
	cfg.BaseURL = "http://example.com"
	errs := validate([]sources.SourceConfig{cfg})
	assertNoErrorField(t, errs, "sources[0].base_url")
}

func TestValidator_BaseURLInvalidScheme(t *testing.T) {
	cases := []string{"ftp://example.com", "file:///tmp", "ws://example.com"}
	for _, u := range cases {
		t.Run(u, func(t *testing.T) {
			cfg := validSource()
			cfg.BaseURL = u
			errs := validate([]sources.SourceConfig{cfg})
			assertHasErrorField(t, errs, "sources[0].base_url")
		})
	}
}

func TestValidator_BaseURLNoScheme(t *testing.T) {
	cfg := validSource()
	cfg.BaseURL = "example.com/manga"
	errs := validate([]sources.SourceConfig{cfg})
	assertHasErrorField(t, errs, "sources[0].base_url")
}

func TestValidator_BaseURLWithPath(t *testing.T) {
	cfg := validSource()
	cfg.BaseURL = "https://example.com/manga/section"
	errs := validate([]sources.SourceConfig{cfg})
	assertNoErrorField(t, errs, "sources[0].base_url")
}

// ── scraper (T-15.3) ─────────────────────────────────────────────────────────

func TestValidator_ScraperRequired(t *testing.T) {
	cfg := validSource()
	cfg.Scraper = ""
	errs := validate([]sources.SourceConfig{cfg})
	assertHasErrorField(t, errs, "sources[0].scraper")
}

func TestValidator_ScraperBuiltinName_NoKnownListSkipsCheck(t *testing.T) {
	// When KnownScrapers is empty, any non-path name is accepted.
	cfg := validSource()
	cfg.Scraper = "anything_goes"
	errs := validate([]sources.SourceConfig{cfg}) // no knownScrapers
	assertNoErrorField(t, errs, "sources[0].scraper")
}

func TestValidator_ScraperBuiltinName_KnownListValid(t *testing.T) {
	cfg := validSource()
	cfg.Scraper = "truyenqq"
	errs := validate([]sources.SourceConfig{cfg}, "truyenqq", "mangadex", "generic")
	assertNoErrorField(t, errs, "sources[0].scraper")
}

func TestValidator_ScraperBuiltinName_KnownListUnknown(t *testing.T) {
	cfg := validSource()
	cfg.Scraper = "nonexistent_scraper"
	errs := validate([]sources.SourceConfig{cfg}, "truyenqq", "mangadex", "generic")
	assertHasErrorField(t, errs, "sources[0].scraper")
}

func TestValidator_ScraperFilePath_NotExists(t *testing.T) {
	cfg := validSource()
	cfg.Scraper = "./scrapers/nonexistent.py"
	errs := validate([]sources.SourceConfig{cfg})
	assertHasErrorField(t, errs, "sources[0].scraper")
}

func TestValidator_ScraperFilePath_Exists(t *testing.T) {
	// Create a temp file to act as the plugin.
	tmp := t.TempDir() + "/custom_scraper.py"
	if err := writeFileDirect(tmp, "# custom scraper"); err != nil {
		t.Fatal(err)
	}
	cfg := validSource()
	cfg.Scraper = tmp // absolute path
	errs := validate([]sources.SourceConfig{cfg})
	assertNoErrorField(t, errs, "sources[0].scraper")
}

// ── rate_limit_ms (T-15.4) ───────────────────────────────────────────────────

func TestValidator_NoRateLimitWarning_HTMLScraper(t *testing.T) {
	cfg := validSource()
	cfg.RateLimitMs = 0
	cfg.Scraper = "truyenqq"
	errs := validate([]sources.SourceConfig{cfg})
	assertHasWarningField(t, errs, "sources[0].rate_limit_ms")
}

func TestValidator_NoRateLimitNoWarning_APIScraper(t *testing.T) {
	// MangaDex is API-based; no warning expected even without rate limiting.
	cfg := validSource()
	cfg.Code = "mangadex"
	cfg.Scraper = "mangadex"
	cfg.RateLimitMs = 0
	errs := validate([]sources.SourceConfig{cfg})
	assertNoWarningField(t, errs, "sources[0].rate_limit_ms")
}

func TestValidator_RateLimitSet_NoWarning(t *testing.T) {
	cfg := validSource()
	cfg.RateLimitMs = 500
	errs := validate([]sources.SourceConfig{cfg})
	assertNoWarningField(t, errs, "sources[0].rate_limit_ms")
}

// ── generic scraper ──────────────────────────────────────────────────────────

func TestValidator_GenericScraper_RequiresSelectors(t *testing.T) {
	cfg := validSource()
	cfg.Scraper = "generic"
	cfg.Selectors = nil
	errs := validate([]sources.SourceConfig{cfg})
	assertHasErrorField(t, errs, "sources[0].selectors")
}

func TestValidator_GenericScraper_WithSelectors_NoError(t *testing.T) {
	cfg := validSource()
	cfg.Scraper = "generic"
	cfg.RateLimitMs = 300
	cfg.Selectors = &sources.Selectors{
		ChapterList:   "ul li a",
		ChapterNumber: "attr:data-ch",
		ImageList:     "div img",
		ImageURL:      "attr:src",
	}
	errs := validate([]sources.SourceConfig{cfg})
	assertNoErrorField(t, errs, "sources[0].selectors")
}

// ── multi-source error isolation ─────────────────────────────────────────────

func TestValidator_MultipleSourcesIndependentErrors(t *testing.T) {
	// Bad code at index 0, missing name at index 1 — both should be reported.
	a := validSource()
	a.Code = "Bad-Code"

	b := validSource()
	b.Code = "source_b"
	b.Name = ""

	errs := validate([]sources.SourceConfig{a, b})
	assertHasErrorField(t, errs, "sources[0].code")
	assertHasErrorField(t, errs, "sources[1].name")
}

func TestValidator_ValidSource_NoErrors(t *testing.T) {
	cfg := validSource()
	errs := validate([]sources.SourceConfig{cfg})
	if errs.HasErrors() {
		t.Errorf("expected no errors for valid source, got: %v", errs.Errors())
	}
}

// ── Assertion helpers ────────────────────────────────────────────────────────

func assertHasErrorField(t *testing.T, errs config.ValidationErrors, field string) {
	t.Helper()
	for _, e := range errs {
		if e.Field == field && e.Severity == config.SeverityError {
			return
		}
	}
	t.Errorf("expected error for field %q; got: %v", field, errs)
}

func assertNoErrorField(t *testing.T, errs config.ValidationErrors, field string) {
	t.Helper()
	for _, e := range errs {
		if e.Field == field && e.Severity == config.SeverityError {
			t.Errorf("unexpected error for field %q: %s", field, e.Message)
		}
	}
}

func assertHasWarningField(t *testing.T, errs config.ValidationErrors, field string) {
	t.Helper()
	for _, e := range errs {
		if e.Field == field && e.Severity == config.SeverityWarning {
			return
		}
	}
	t.Errorf("expected warning for field %q; got: %v", field, errs)
}

func assertNoWarningField(t *testing.T, errs config.ValidationErrors, field string) {
	t.Helper()
	for _, e := range errs {
		if e.Field == field && e.Severity == config.SeverityWarning {
			t.Errorf("unexpected warning for field %q: %s", field, e.Message)
		}
	}
}

func writeFileDirect(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
