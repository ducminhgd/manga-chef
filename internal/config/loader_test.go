package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ducminhgd/manga-chef/internal/config"
	"github.com/ducminhgd/manga-chef/pkg/sources"
)

// testdataPath returns an absolute path to a file in the testdata directory.
func testdataPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", name)
}

// ── LoadFile ─────────────────────────────────────────────────────────────────

func TestLoadFile_ValidSingle(t *testing.T) {
	cfgs, err := config.LoadFile(testdataPath(t, "valid_single.yml"))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("expected 1 source, got %d", len(cfgs))
	}

	got := cfgs[0]
	assertEqual(t, "Name", "TruyenQQ", got.Name)
	assertEqual(t, "Code", "truyenqq", got.Code)
	assertEqual(t, "BaseURL", "https://truyenqqno.com", got.BaseURL)
	assertEqual(t, "Scraper", "truyenqq", got.Scraper)
	assertEqual(t, "RateLimitMs", 500, got.RateLimitMs)
	assertEqual(t, "Retries", 3, got.Retries)
	assertEqual(t, "TimeoutS", 30, got.TimeoutS)

	if got.Headers["Referer"] != "https://truyenqqno.com" {
		t.Errorf("Headers[Referer]: expected %q, got %q", "https://truyenqqno.com", got.Headers["Referer"])
	}
}

func TestLoadFile_ValidMultiple(t *testing.T) {
	cfgs, err := config.LoadFile(testdataPath(t, "valid_multi.yml"))
	if err != nil {
		// Allow warnings through — check they are only warnings.
		var ve config.ValidationErrors
		if !errors.As(err, &ve) || ve.HasErrors() {
			t.Fatalf("expected no hard errors, got: %v", err)
		}
	}
	if len(cfgs) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(cfgs))
	}

	codes := []string{"truyenqq", "mangadex", "generic_site"}
	for i, want := range codes {
		if cfgs[i].Code != want {
			t.Errorf("cfgs[%d].Code: expected %q, got %q", i, want, cfgs[i].Code)
		}
	}
}

func TestLoadFile_GenericScraperHasSelectors(t *testing.T) {
	cfgs, err := config.LoadFile(testdataPath(t, "valid_multi.yml"))
	// Only care about the generic_site entry (index 2).
	var ve config.ValidationErrors
	if err != nil && (!errors.As(err, &ve) || ve.HasErrors()) {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if len(cfgs) < 3 {
		t.Fatalf("expected at least 3 sources")
	}
	generic := cfgs[2]
	if generic.Selectors == nil {
		t.Fatal("expected selectors to be populated, got nil")
	}
	assertEqual(t, "Selectors.ChapterList", "ul.chapters li a", generic.Selectors.ChapterList)
	assertEqual(t, "Selectors.ImageURL", "attr:src", generic.Selectors.ImageURL)
}

func TestLoadFile_EmptyFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "empty.yml")
	if err := os.WriteFile(f, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgs, err := config.LoadFile(f)
	if err != nil {
		t.Fatalf("empty file should produce no error, got: %v", err)
	}
	if len(cfgs) != 0 {
		t.Errorf("expected 0 sources from empty file, got %d", len(cfgs))
	}
}

func TestLoadFile_FileNotFound(t *testing.T) {
	_, err := config.LoadFile("/nonexistent/path/sources.yml")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestLoadFile_BadYAMLSyntax(t *testing.T) {
	_, err := config.LoadFile(testdataPath(t, "bad_syntax.yml"))
	if err == nil {
		t.Fatal("expected parse error for malformed YAML, got nil")
	}
	// Must NOT be a ValidationErrors — it's a hard parse error.
	var ve config.ValidationErrors
	if errors.As(err, &ve) {
		t.Errorf("expected a parse error, not a ValidationErrors: %v", err)
	}
}

func TestLoadFile_DuplicateCode(t *testing.T) {
	_, err := config.LoadFile(testdataPath(t, "duplicate_code.yml"))
	assertValidationError(t, err, "sources[1].code")
}

func TestLoadFile_InvalidCodeChars(t *testing.T) {
	_, err := config.LoadFile(testdataPath(t, "invalid_code_chars.yml"))
	assertValidationError(t, err, "sources[0].code")
	assertValidationError(t, err, "sources[1].code")
	assertValidationError(t, err, "sources[2].code")
}

func TestLoadFile_InvalidURL(t *testing.T) {
	_, err := config.LoadFile(testdataPath(t, "invalid_url.yml"))
	assertValidationError(t, err, "sources[0].base_url")
	assertValidationError(t, err, "sources[1].base_url")
}

func TestLoadFile_MissingRequiredFields(t *testing.T) {
	_, err := config.LoadFile(testdataPath(t, "missing_required.yml"))
	assertValidationError(t, err, "sources[0].name")
	assertValidationError(t, err, "sources[0].code")
	assertValidationError(t, err, "sources[0].base_url")
	assertValidationError(t, err, "sources[0].scraper")
}

func TestLoadFile_GenericScraperMissingSelectors(t *testing.T) {
	_, err := config.LoadFile(testdataPath(t, "generic_no_selectors.yml"))
	assertValidationError(t, err, "sources[0].selectors")
}

func TestLoadFile_NoRateLimitProducesWarning(t *testing.T) {
	_, err := config.LoadFile(testdataPath(t, "no_rate_limit.yml"))
	if err == nil {
		t.Fatal("expected a ValidationErrors with a warning, got nil")
	}
	var ve config.ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationErrors, got %T: %v", err, err)
	}
	if ve.HasErrors() {
		t.Errorf("expected warnings only, but got hard errors: %v", ve.Errors())
	}
	if len(ve.Warnings()) == 0 {
		t.Error("expected at least one warning about missing rate_limit_ms")
	}
}

func TestLoadFile_ErrorsContainLineNumbers(t *testing.T) {
	_, err := config.LoadFile(testdataPath(t, "invalid_code_chars.yml"))
	var ve config.ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	for _, e := range ve.Errors() {
		if e.Line == 0 {
			t.Errorf("expected Line > 0 for error %q, got 0", e.Field)
		}
	}
}

// ── LoadDir ──────────────────────────────────────────────────────────────────

func TestLoadDir_LoadsAllYMLFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.yml"), sourceYAML("Source A", "source_a", "https://a.com", "generic_a", 300))
	writeFile(t, filepath.Join(dir, "b.yml"), sourceYAML("Source B", "source_b", "https://b.com", "generic_b", 300))
	writeFile(t, filepath.Join(dir, "c.txt"), "not a yaml file")

	cfgs, err := config.LoadDir(dir)
	if err != nil {
		var ve config.ValidationErrors
		if !errors.As(err, &ve) || ve.HasErrors() {
			t.Fatalf("unexpected hard error: %v", err)
		}
	}
	if len(cfgs) != 2 {
		t.Errorf("expected 2 sources from 2 yml files, got %d", len(cfgs))
	}
}

func TestLoadDir_CrossFileDuplicateCode(t *testing.T) {
	dir := t.TempDir()
	sameCode := sourceYAML("Source A", "duplicate_code", "https://a.com", "scraper_a", 300)
	writeFile(t, filepath.Join(dir, "a.yml"), sameCode)
	writeFile(t, filepath.Join(dir, "b.yml"), sourceYAML("Source B", "duplicate_code", "https://b.com", "scraper_b", 300))

	_, err := config.LoadDir(dir)
	if err == nil {
		t.Fatal("expected error for cross-file duplicate code, got nil")
	}
	var ve config.ValidationErrors
	if !errors.As(err, &ve) || !ve.HasErrors() {
		t.Fatalf("expected ValidationErrors with errors, got: %v", err)
	}
}

func TestLoadDir_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	cfgs, err := config.LoadDir(dir)
	if err != nil {
		t.Fatalf("empty directory should produce no error, got: %v", err)
	}
	if len(cfgs) != 0 {
		t.Errorf("expected 0 sources from empty directory, got %d", len(cfgs))
	}
}

func TestLoadDir_NotFound(t *testing.T) {
	_, err := config.LoadDir("/nonexistent/sources/dir")
	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// ── Load (auto-detect file vs directory) ─────────────────────────────────────

func TestLoad_DispatchesToLoadFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "sources.yml")
	writeFile(t, f, sourceYAML("TruyenQQ", "truyenqq", "https://truyenqqno.com", "truyenqq", 500))

	cfgs, err := config.Load(f)
	if err != nil {
		var ve config.ValidationErrors
		if !errors.As(err, &ve) || ve.HasErrors() {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if len(cfgs) != 1 {
		t.Errorf("expected 1 source, got %d", len(cfgs))
	}
}

func TestLoad_DispatchesToLoadDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "sources.yml"), sourceYAML("TruyenQQ", "truyenqq", "https://truyenqqno.com", "truyenqq", 500))

	cfgs, err := config.Load(dir)
	if err != nil {
		var ve config.ValidationErrors
		if !errors.As(err, &ve) || ve.HasErrors() {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if len(cfgs) != 1 {
		t.Errorf("expected 1 source, got %d", len(cfgs))
	}
}

// ── SourceConfig helpers ─────────────────────────────────────────────────────

func TestSourceConfig_IsEnabled_DefaultTrue(t *testing.T) {
	cfg := sources.SourceConfig{}
	if !cfg.IsEnabled() {
		t.Error("IsEnabled() should return true when Enabled field is nil")
	}
}

func TestSourceConfig_IsEnabled_ExplicitFalse(t *testing.T) {
	f := false
	cfg := sources.SourceConfig{Enabled: &f}
	if cfg.IsEnabled() {
		t.Error("IsEnabled() should return false when Enabled is explicitly false")
	}
}

func TestSourceConfig_EffectiveRetries_Default(t *testing.T) {
	cfg := sources.SourceConfig{}
	if cfg.EffectiveRetries() != sources.DefaultRetries {
		t.Errorf("EffectiveRetries() = %d, want %d", cfg.EffectiveRetries(), sources.DefaultRetries)
	}
}

func TestSourceConfig_EffectiveTimeoutS_Default(t *testing.T) {
	cfg := sources.SourceConfig{}
	if cfg.EffectiveTimeoutS() != sources.DefaultTimeoutS {
		t.Errorf("EffectiveTimeoutS() = %d, want %d", cfg.EffectiveTimeoutS(), sources.DefaultTimeoutS)
	}
}

// ── Test helpers ─────────────────────────────────────────────────────────────

// assertEqual is a typed equality helper that produces a clear diff on failure.
func assertEqual[T comparable](t *testing.T, field string, want, got T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: expected %v, got %v", field, want, got)
	}
}

// assertValidationError fails the test if err does not contain a ValidationError
// whose Field matches the given field path.
func assertValidationError(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Errorf("expected validation error for field %q, got nil", field)
		return
	}
	var ve config.ValidationErrors
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationErrors, got %T: %v", err, err)
		return
	}
	for _, e := range ve {
		if e.Field == field && e.Severity == config.SeverityError {
			return
		}
	}
	t.Errorf("expected error for field %q; got errors: %v", field, ve)
}

// sourceYAML generates a minimal valid sources YAML string for use in tests
// that only need syntactically correct YAML without loading fixture files.
func sourceYAML(name, code, baseURL, scraper string, rateLimitMs int) string {
	return "sources:\n" +
		"  - name: \"" + name + "\"\n" +
		"    code: \"" + code + "\"\n" +
		"    base_url: \"" + baseURL + "\"\n" +
		"    scraper: \"" + scraper + "\"\n" +
		"    rate_limit_ms: " + itoa(rateLimitMs) + "\n"
}

// writeFile creates a file at path with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeFile(%q): %v", path, err)
	}
}

// itoa converts an int to its decimal string representation
// without importing strconv at the package level just for tests.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
