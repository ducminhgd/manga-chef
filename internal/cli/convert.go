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
			outputPath := strings.TrimSpace(getOutputPath())
			if strings.TrimSpace(inputDir) == "" {
				return errors.New("--input is required")
			}
			if outputPath == "" {
				return errors.New("--output is required")
			}
			if strings.TrimSpace(format) == "" {
				return errors.New("--format is required")
			}
			formats, err := parseFormats(format)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			inputKind, err := convertInputKind(inputDir)
			if err != nil {
				return err
			}
			if inputKind == "chapter" {
				for _, format := range formats {
					conv, err := newConverterByFormat(format)
					if err != nil {
						return err
					}
					target := resolveChapterOutputPath(outputPath, format, len(formats))
					if err := conv.Convert(ctx, inputDir, target, converter.Options{Title: title}); err != nil {
						return fmt.Errorf("convert failed: %w", err)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Converted %s -> %s (%s)\n", filepath.Clean(inputDir), target, strings.ToLower(format))
				}
				return nil
			}

			limits := mergeLimits{
				MaxFileSizeMB: maxSizeMB,
				MaxPages:      maxPages,
				MaxChapters:   maxChapters,
			}
			for _, format := range formats {
				conv, err := newConverterByFormat(format)
				if err != nil {
					return err
				}
				if err := convertMangaRoot(ctx, conv, inputDir, outputPath, format, title, limits, cmd.OutOrStdout()); err != nil {
					return fmt.Errorf("convert failed: %w", err)
				}
			}
			return nil
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
