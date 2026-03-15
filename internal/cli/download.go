package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ducminhgd/manga-chef/internal/config"
	"github.com/ducminhgd/manga-chef/internal/downloader"
	"github.com/ducminhgd/manga-chef/internal/scraper"
	"github.com/ducminhgd/manga-chef/pkg/sources"
)

func newDownloadCmd(getSourcesPath func() string, getOutputDir func() string) *cobra.Command {
	var sourceCode string
	var mangaURL string
	var workers int
	var force bool
	var quiet bool
	var fromChapter float64
	var toChapter float64

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download chapters from a source",
		Long: `Download manga chapters from a configured source. The source is selected by code from your sources file.
Example: manga-chef download --source truyenqq --url https://truyenqqno.com/truyen-tranh/dau-an-rong-thieng-236`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if fromChapter > 0 && toChapter > 0 && fromChapter > toChapter {
				return errors.New("--from cannot be greater than --to")
			}
			if sourceCode == "" {
				return errors.New("--source is required")
			}
			if mangaURL == "" {
				return errors.New("--url is required")
			}
			sourcesPath := getSourcesPath()
			cfgs, err := config.Load(sourcesPath)
			if err != nil {
				return fmt.Errorf("loading sources from %q: %w", sourcesPath, err)
			}
			srcCfg, err := findSourceByCode(cfgs, sourceCode)
			if err != nil {
				return err
			}
			if !srcCfg.IsEnabled() {
				return fmt.Errorf("source %q is disabled", sourceCode)
			}

			scr, err := scraper.Get(srcCfg.Scraper, &srcCfg)
			if err != nil {
				return fmt.Errorf("constructing scraper %q: %w", srcCfg.Scraper, err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			chapters, err := scr.GetChapterList(ctx, mangaURL)
			if err != nil {
				return fmt.Errorf("getting chapter list: %w", err)
			}
			if len(chapters) == 0 {
				return errors.New("no chapters found")
			}
			chapters = filterChaptersByRange(chapters, fromChapter, toChapter)
			if len(chapters) == 0 {
				return errors.New("no chapters found in requested range")
			}

			outDir := outputDirectory(getOutputDir(), mangaURL)

			reporter := (downloader.ProgressReporter)(nil)
			if !quiet {
				reporter = downloader.NewTerminalProgressReporter(cmd.OutOrStdout())
			}
			d, err := downloader.New(scr, downloader.Options{
				OutDir:            outDir,
				Workers:           workers,
				Force:             force,
				RateLimitMs:       srcCfg.RateLimitMs,
				Retries:           srcCfg.EffectiveRetries(),
				RequestTimeoutSec: srcCfg.EffectiveTimeoutS(),
				Headers:           srcCfg.Headers,
				Reporter:          reporter,
			})
			if err != nil {
				return fmt.Errorf("creating downloader: %w", err)
			}

			if err := d.DownloadManga(ctx, sourceCode, chapters); err != nil {
				return fmt.Errorf("download failed: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %d chapters to %s\n", len(chapters), outDir)
			return nil
		},
	}

	cmd.Flags().StringVar(&sourceCode, "source", "", "Source code (e.g. truyenqq)")
	cmd.Flags().StringVar(&mangaURL, "url", "", "Manga URL to download")
	cmd.Flags().Float64Var(&fromChapter, "from", 0, "Start chapter number (inclusive)")
	cmd.Flags().Float64Var(&toChapter, "to", 0, "End chapter number (inclusive)")
	cmd.Flags().IntVar(&workers, "workers", 4, "Number of concurrent image downloader workers")
	cmd.Flags().BoolVar(&force, "force", false, "Force re-download chapters even if already present")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress progress output")
	return cmd
}

func filterChaptersByRange(chapters []sources.Chapter, from, to float64) []sources.Chapter {
	if from <= 0 && to <= 0 {
		return chapters
	}
	filtered := make([]sources.Chapter, 0, len(chapters))
	for _, ch := range chapters {
		if from > 0 && ch.Number < from {
			continue
		}
		if to > 0 && ch.Number > to {
			continue
		}
		filtered = append(filtered, ch)
	}
	return filtered
}

func outputDirectory(outputFlag, mangaURL string) string {
	if strings.TrimSpace(outputFlag) != "" {
		return outputFlag
	}
	u, err := url.Parse(mangaURL)
	if err != nil {
		return "manga"
	}
	slug := strings.Trim(u.Path, "/")
	if slug == "" {
		return "manga"
	}
	parts := strings.Split(slug, "/")
	name := parts[len(parts)-1]
	if name == "" {
		return "manga"
	}
	return sanitizeSlug(name)
}

func sanitizeSlug(input string) string {
	clean := strings.TrimSpace(input)
	if clean == "" {
		return "manga"
	}
	clean = strings.ToLower(clean)
	clean = strings.ReplaceAll(clean, " ", "-")
	for _, c := range clean {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' {
			continue
		}
		clean = strings.ReplaceAll(clean, string(c), "-")
	}
	clean = strings.Trim(clean, "-_.")
	if clean == "" {
		return "manga"
	}
	return clean
}

func findSourceByCode(cfgs []sources.SourceConfig, code string) (sources.SourceConfig, error) {
	for _, c := range cfgs {
		if strings.EqualFold(c.Code, code) {
			return c, nil
		}
	}
	return sources.SourceConfig{}, fmt.Errorf("source %q not found in configuration", code)
}
