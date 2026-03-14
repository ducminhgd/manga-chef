package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ducminhgd/manga-chef/internal/cli"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// runSources executes a "sources" sub-command against a given sources path
// and returns the combined stdout output and any error.
func runSources(t *testing.T, sourcesPath string, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	root := cli.NewSourcesCmd(func() string { return sourcesPath })
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// writeSource writes a minimal single-source YAML file to path.
func writeSource(t *testing.T, path, name, code, baseURL, scraper string, rateLimitMs int) {
	t.Helper()
	content := "sources:\n" +
		"  - name: \"" + name + "\"\n" +
		"    code: \"" + code + "\"\n" +
		"    base_url: \"" + baseURL + "\"\n" +
		"    scraper: \"" + scraper + "\"\n" +
		"    rate_limit_ms: " + itoa(rateLimitMs) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeSource(%q): %v", path, err)
	}
}

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

// ── sources list (T-17) ──────────────────────────────────────────────────────

func TestSourcesList_ShowsAllEnabledSources(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, filepath.Join(dir, "truyenqq.yml"),
		"TruyenQQ", "truyenqq", "https://truyenqqto.com", "truyenqq", 500)
	writeSource(t, filepath.Join(dir, "mangadex.yml"),
		"MangaDex", "mangadex", "https://api.mangadex.org", "mangadex", 0)

	out, err := runSources(t, dir, "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, out, "truyenqq")
	assertContains(t, out, "TruyenQQ")
	assertContains(t, out, "https://truyenqqto.com")
	assertContains(t, out, "mangadex")
}

func TestSourcesList_HidesDisabledByDefault(t *testing.T) {
	dir := t.TempDir()
	content := `sources:
  - name: "Active"
    code: "active"
    base_url: "https://active.com"
    scraper: "active"
    rate_limit_ms: 300
  - name: "Disabled"
    code: "disabled_src"
    base_url: "https://disabled.com"
    scraper: "disabled_src"
    rate_limit_ms: 300
    enabled: false
`
	if err := os.WriteFile(filepath.Join(dir, "sources.yml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := runSources(t, dir, "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, out, "active")
	assertNotContains(t, out, "disabled_src")
}

func TestSourcesList_ShowsDisabledWithAllFlag(t *testing.T) {
	dir := t.TempDir()
	content := `sources:
  - name: "Disabled"
    code: "disabled_src"
    base_url: "https://disabled.com"
    scraper: "disabled_src"
    rate_limit_ms: 300
    enabled: false
`
	if err := os.WriteFile(filepath.Join(dir, "sources.yml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := runSources(t, dir, "list", "--all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, out, "disabled_src")
	assertContains(t, out, "disabled")
}

func TestSourcesList_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	out, err := runSources(t, dir, "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, out, "No sources found")
}

func TestSourcesList_TableHasExpectedColumns(t *testing.T) {
	dir := t.TempDir()
	writeSource(t, filepath.Join(dir, "s.yml"),
		"TruyenQQ", "truyenqq", "https://truyenqqto.com", "truyenqq", 500)

	out, err := runSources(t, dir, "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, col := range []string{"CODE", "NAME", "BASE URL", "SCRAPER", "STATUS"} {
		assertContains(t, out, col)
	}
}

func TestSourcesList_InvalidConfig_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	bad := `sources:
  - name: ""
    code: "Bad Code"
    base_url: "not-a-url"
    scraper: "x"
    rate_limit_ms: 300
`
	if err := os.WriteFile(filepath.Join(dir, "bad.yml"), []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := runSources(t, dir, "list")
	if err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
}

// ── sources add (T-18) ────────────────────────────────────────────────────────

func TestSourcesAdd_AddsNewSource(t *testing.T) {
	destDir := t.TempDir()
	srcDir := t.TempDir()
	newFile := filepath.Join(srcDir, "truyenqq.yml")
	writeSource(t, newFile, "TruyenQQ", "truyenqq", "https://truyenqqto.com", "truyenqq", 500)

	out, err := runSources(t, destDir, "add", newFile)
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out)
	}

	assertContains(t, out, "truyenqq")

	// Verify the file was copied into the destination directory.
	copied := filepath.Join(destDir, "truyenqq.yml")
	if _, statErr := os.Stat(copied); os.IsNotExist(statErr) {
		t.Errorf("expected file to be copied to %q, but it does not exist", copied)
	}
}

func TestSourcesAdd_DuplicateCode_Errors(t *testing.T) {
	destDir := t.TempDir()
	srcDir := t.TempDir()

	// Put an existing source in destDir.
	writeSource(t, filepath.Join(destDir, "existing.yml"),
		"TruyenQQ", "truyenqq", "https://truyenqqto.com", "truyenqq", 500)

	// Try to add another file with the same code.
	newFile := filepath.Join(srcDir, "duplicate.yml")
	writeSource(t, newFile, "TruyenQQ Mirror", "truyenqq", "https://mirror.com", "truyenqq", 500)

	_, err := runSources(t, destDir, "add", newFile)
	if err == nil {
		t.Fatal("expected error for duplicate source code, got nil")
	}
	if !strings.Contains(err.Error(), "truyenqq") {
		t.Errorf("error should mention the duplicate code; got: %v", err)
	}
}

func TestSourcesAdd_DryRun_DoesNotWriteFile(t *testing.T) {
	destDir := t.TempDir()
	srcDir := t.TempDir()
	newFile := filepath.Join(srcDir, "truyenqq.yml")
	writeSource(t, newFile, "TruyenQQ", "truyenqq", "https://truyenqqto.com", "truyenqq", 500)

	out, err := runSources(t, destDir, "add", newFile, "--dry-run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, out, "Dry run")

	// File must NOT have been copied.
	copied := filepath.Join(destDir, "truyenqq.yml")
	if _, statErr := os.Stat(copied); !os.IsNotExist(statErr) {
		t.Error("--dry-run should not have written any files")
	}
}

func TestSourcesAdd_InvalidFile_ReturnsError(t *testing.T) {
	destDir := t.TempDir()
	srcDir := t.TempDir()
	bad := filepath.Join(srcDir, "bad.yml")
	if err := os.WriteFile(bad, []byte(`sources:
  - name: ""
    code: "Bad-Code!"
    base_url: "not-a-url"
    scraper: "x"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := runSources(t, destDir, "add", bad)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestSourcesAdd_EmptyFile_Noop(t *testing.T) {
	destDir := t.TempDir()
	srcDir := t.TempDir()
	empty := filepath.Join(srcDir, "empty.yml")
	if err := os.WriteFile(empty, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := runSources(t, destDir, "add", empty)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, out, "nothing to add")
}

func TestSourcesAdd_MissingArgument_ReturnsError(t *testing.T) {
	_, err := runSources(t, t.TempDir(), "add")
	if err == nil {
		t.Fatal("expected error when no file argument is given")
	}
}

func TestSourcesAdd_CreatesDestDirIfMissing(t *testing.T) {
	parent := t.TempDir()
	destDir := filepath.Join(parent, "new-sources-dir") // does not exist yet
	srcDir := t.TempDir()
	newFile := filepath.Join(srcDir, "truyenqq.yml")
	writeSource(t, newFile, "TruyenQQ", "truyenqq", "https://truyenqqto.com", "truyenqq", 500)

	_, err := runSources(t, destDir, "add", newFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, statErr := os.Stat(destDir); os.IsNotExist(statErr) {
		t.Error("expected destination directory to be created")
	}
}

// ── assertion helpers ────────────────────────────────────────────────────────

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("output should contain %q\ngot:\n%s", want, got)
	}
}

func assertNotContains(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Errorf("output should NOT contain %q\ngot:\n%s", want, got)
	}
}
