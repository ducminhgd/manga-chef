package mobi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	epubconverter "github.com/ducminhgd/manga-chef/internal/converter/epub"

	"github.com/ducminhgd/manga-chef/internal/converter"
)

const (
	calibreBinary = "ebook-convert"
	calibreURL    = "https://calibre-ebook.com/download"
)

// Converter converts a chapter image folder into MOBI by first generating EPUB,
// then invoking Calibre's ebook-convert CLI.
type Converter struct {
	epub     converter.ConverterInterface
	lookPath func(file string) (string, error)
	run      func(ctx context.Context, name string, args ...string) error
}

// New returns a MOBI converter implementation.
func New() *Converter {
	return &Converter{
		epub:     epubconverter.New(),
		lookPath: exec.LookPath,
		run:      runCommand,
	}
}

// EnsureAvailable checks that Calibre's ebook-convert is available on PATH.
func (c *Converter) EnsureAvailable() error {
	_, err := c.lookPath(calibreBinary)
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) || strings.Contains(strings.ToLower(err.Error()), "not found") {
		return fmt.Errorf("%s not found on PATH; install Calibre from %s", calibreBinary, calibreURL)
	}
	return fmt.Errorf("checking %s availability: %w", calibreBinary, err)
}

// Convert creates a temporary EPUB from inputDir, then converts it to MOBI.
func (c *Converter) Convert(ctx context.Context, inputDir, outputPath string, opts converter.Options) error {
	if strings.TrimSpace(inputDir) == "" {
		return errors.New("input directory is required")
	}
	if strings.TrimSpace(outputPath) == "" {
		return errors.New("output path is required")
	}
	if err := c.EnsureAvailable(); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "manga-chef-mobi-*")
	if err != nil {
		return fmt.Errorf("creating temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpEpub := filepath.Join(tmpDir, "intermediate.epub")
	if err := c.epub.Convert(ctx, inputDir, tmpEpub, opts); err != nil {
		return fmt.Errorf("converting source images to temporary epub: %w", err)
	}

	if err := c.run(ctx, calibreBinary, tmpEpub, outputPath); err != nil {
		return fmt.Errorf("running %s %q %q: %w", calibreBinary, tmpEpub, outputPath, err)
	}
	return nil
}

func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return fmt.Errorf("running %s: %w", name, err)
	}
	return nil
}
