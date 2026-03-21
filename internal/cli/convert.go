package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ducminhgd/manga-chef/internal/converter"
	epubconverter "github.com/ducminhgd/manga-chef/internal/converter/epub"
	mobiconverter "github.com/ducminhgd/manga-chef/internal/converter/mobi"
	pdfconverter "github.com/ducminhgd/manga-chef/internal/converter/pdf"
)

func newConvertCmd(getOutputPath func() string) *cobra.Command {
	var inputDir string
	var format string
	var title string
	var maxSizeMB int
	var maxPages int
	var maxChapters int

	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert chapter directory or manga root directory into output format",
		Long: `Convert manga images into output files.

If --input points to a chapter directory (contains images), a single file is produced.
If --input points to a manga root (contains chapter sub-directories), output is split
into one or more volume files based on merge limits.`,
		Example: "manga-chef convert --input ./out/truyenqq/chap-001 --format epub --output ./out/chapter-001.epub",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConvertCmd(cmd, &convertCommandOptions{
				inputDir:    inputDir,
				outputPath:  strings.TrimSpace(getOutputPath()),
				format:      format,
				title:       title,
				maxSizeMB:   maxSizeMB,
				maxPages:    maxPages,
				maxChapters: maxChapters,
			})
		},
	}

	cmd.Flags().StringVar(&inputDir, "input", "", "Input directory containing chapter images")
	cmd.Flags().StringVar(&format, "format", "pdf", "Output format (supported: pdf, epub, mobi)")
	cmd.Flags().StringVar(&title, "title", "", "Optional document title metadata")
	cmd.Flags().IntVar(&maxSizeMB, "max-size-mb", defaultMaxFileSizeMB, "Maximum output size per volume in MB (-1 to disable)")
	cmd.Flags().IntVar(&maxPages, "max-pages", defaultMaxPages, "Maximum pages per volume (-1 to disable)")
	cmd.Flags().IntVar(&maxChapters, "max-chapters", defaultMaxChapters, "Maximum chapters per volume (-1 to disable)")
	return cmd
}

type convertCommandOptions struct {
	inputDir    string
	outputPath  string
	format      string
	title       string
	maxSizeMB   int
	maxPages    int
	maxChapters int
}

func runConvertCmd(cmd *cobra.Command, opts *convertCommandOptions) error {
	if err := validateConvertOptions(opts); err != nil {
		return err
	}

	formats, err := parseFormats(opts.format)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	inputKind, err := convertInputKind(opts.inputDir)
	if err != nil {
		return err
	}
	if inputKind == "chapter" {
		return convertSingleChapter(ctx, cmd, opts, formats)
	}

	limits := mergeLimits{
		MaxFileSizeMB: opts.maxSizeMB,
		MaxPages:      opts.maxPages,
		MaxChapters:   opts.maxChapters,
	}
	return convertMangaVolumes(ctx, cmd, opts, formats, limits)
}

func validateConvertOptions(opts *convertCommandOptions) error {
	if strings.TrimSpace(opts.inputDir) == "" {
		return errors.New("--input is required")
	}
	if opts.outputPath == "" {
		return errors.New("--output is required")
	}
	if strings.TrimSpace(opts.format) == "" {
		return errors.New("--format is required")
	}
	return nil
}

func convertSingleChapter(ctx context.Context, cmd *cobra.Command, opts *convertCommandOptions, formats []string) error {
	for _, format := range formats {
		conv, err := newConverterByFormat(format)
		if err != nil {
			return err
		}
		target := resolveChapterOutputPath(opts.outputPath, format, len(formats))
		if err := conv.Convert(ctx, opts.inputDir, target, converter.Options{Title: opts.title}); err != nil {
			return fmt.Errorf("convert failed: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Converted %s -> %s (%s)\n", filepath.Clean(opts.inputDir), target, strings.ToLower(format))
	}
	return nil
}

func convertMangaVolumes(ctx context.Context, cmd *cobra.Command, opts *convertCommandOptions, formats []string, limits mergeLimits) error {
	for _, format := range formats {
		conv, err := newConverterByFormat(format)
		if err != nil {
			return err
		}
		if err := convertMangaRoot(ctx, conv, opts.inputDir, opts.outputPath, format, opts.title, limits, cmd.OutOrStdout()); err != nil {
			return fmt.Errorf("convert failed: %w", err)
		}
	}
	return nil
}

func newConverterByFormat(format string) (converter.ConverterInterface, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "pdf":
		return pdfconverter.New(), nil
	case "epub":
		return epubconverter.New(), nil
	case "mobi":
		c := mobiconverter.New()
		if err := c.EnsureAvailable(); err != nil {
			return nil, err
		}
		return c, nil
	default:
		return nil, fmt.Errorf("unsupported format %q (supported: pdf, epub, mobi)", format)
	}
}

func parseFormats(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	formats := make([]string, 0, len(parts))
	for _, part := range parts {
		format := strings.ToLower(strings.TrimSpace(part))
		if format == "" {
			continue
		}
		if !slices.Contains(formats, format) {
			formats = append(formats, format)
		}
	}
	if len(formats) == 0 {
		return nil, errors.New("at least one format is required")
	}
	for _, format := range formats {
		if _, err := newConverterByFormat(format); err != nil {
			return nil, err
		}
	}
	return formats, nil
}

func resolveChapterOutputPath(outputPath, format string, totalFormats int) string {
	cleanOut := filepath.Clean(outputPath)
	if totalFormats <= 1 {
		return cleanOut
	}

	ext := filepath.Ext(cleanOut)
	if ext == "" {
		return cleanOut + "." + strings.ToLower(format)
	}
	base := strings.TrimSuffix(cleanOut, ext)
	return base + "." + strings.ToLower(format)
}
