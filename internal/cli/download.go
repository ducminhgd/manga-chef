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

func newDownloadCmd(getSourcesPath, getOutputDir func() string) *cobra.Command {
	var sourceCode string
	var mangaURL string
	var workers int
	var force bool
	var quiet bool
	var fromChapter float64
	var toChapter float64
	var maxWaitSec int

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download chapters from a source",
		Long: `Download manga chapters from a configured source. The source is selected by code from your sources file.
Example: manga-chef download --source truyenqq --url https://truyenqqno.com/truyen-tranh/dau-an-rong-thieng-236`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDownloadCmd(cmd, &downloadCommandOptions{
				sourceCode:  sourceCode,
				mangaURL:    mangaURL,
				workers:     workers,
				force:       force,
				quiet:       quiet,
				fromChapter: fromChapter,
				toChapter:   toChapter,
				maxWaitSec:  maxWaitSec,
				sourcesPath: getSourcesPath(),
				outputDir:   getOutputDir(),
			})
		},
	}

	cmd.Flags().StringVar(&sourceCode, "source", "", "Source code (e.g. truyenqq)")
	cmd.Flags().StringVar(&mangaURL, "url", "", "Manga URL to download")
	cmd.Flags().Float64Var(&fromChapter, "from", 0, "Start chapter number (inclusive)")
	cmd.Flags().Float64Var(&toChapter, "to", 0, "End chapter number (inclusive)")
	cmd.Flags().IntVar(&workers, "workers", 4, "Number of concurrent image downloader workers")
	cmd.Flags().BoolVar(&force, "force", false, "Force re-download chapters even if already present")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress progress output")
	cmd.Flags().IntVar(&maxWaitSec, "max-wait", 300, "Wait time in seconds after each failed 3-attempt retry cycle before trying again")
	return cmd
}

type downloadCommandOptions struct {
	sourceCode  string
	mangaURL    string
	sourcesPath string
	outputDir   string
	workers     int
	force       bool
	quiet       bool
	fromChapter float64
	toChapter   float64
	maxWaitSec  int
}

func runDownloadCmd(cmd *cobra.Command, opts *downloadCommandOptions) error {
	if err := validateDownloadOptions(opts); err != nil {
		return err
	}

	cfgs, err := config.Load(opts.sourcesPath)
	if err != nil {
		return fmt.Errorf("loading sources from %q: %w", opts.sourcesPath, err)
	}
	srcCfg, err := findSourceByCode(cfgs, opts.sourceCode)
	if err != nil {
		return err
	}
	if !srcCfg.IsEnabled() {
		return fmt.Errorf("source %q is disabled", opts.sourceCode)
	}

	scr, err := scraper.Get(srcCfg.Scraper, &srcCfg)
	if err != nil {
		return fmt.Errorf("constructing scraper %q: %w", srcCfg.Scraper, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	chapters, err := loadChaptersForDownload(ctx, scr, opts)
	if err != nil {
		return err
	}

	outDir := outputDirectory(opts.outputDir, opts.mangaURL)
	d, err := newDownloaderForCommand(cmd, scr, &srcCfg, opts, outDir)
	if err != nil {
		return err
	}
	if err := d.DownloadManga(ctx, opts.sourceCode, chapters); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %d chapters to %s\n", len(chapters), outDir)
	return nil
}

func validateDownloadOptions(opts *downloadCommandOptions) error {
	if opts.fromChapter > 0 && opts.toChapter > 0 && opts.fromChapter > opts.toChapter {
		return errors.New("--from cannot be greater than --to")
	}
	if opts.sourceCode == "" {
		return errors.New("--source is required")
	}
	if opts.mangaURL == "" {
		return errors.New("--url is required")
	}
	return nil
}

func loadChaptersForDownload(ctx context.Context, scr scraper.ScraperInterface, opts *downloadCommandOptions) ([]sources.Chapter, error) {
	chapters, err := scr.GetChapterList(ctx, opts.mangaURL)
	if err != nil {
		return nil, fmt.Errorf("getting chapter list: %w", err)
	}
	if len(chapters) == 0 {
		return nil, errors.New("no chapters found")
	}
	chapters = filterChaptersByRange(chapters, opts.fromChapter, opts.toChapter)
	if len(chapters) == 0 {
		return nil, errors.New("no chapters found in requested range")
	}
	return chapters, nil
}

func newDownloaderForCommand(cmd *cobra.Command, scr scraper.ScraperInterface, srcCfg *sources.SourceConfig, opts *downloadCommandOptions, outDir string) (*downloader.Downloader, error) {
	var reporter downloader.ProgressReporter
	if !opts.quiet {
		reporter = downloader.NewTerminalProgressReporter(cmd.OutOrStdout())
	}

	d, err := downloader.New(scr, &downloader.Options{
		OutDir:            outDir,
		Workers:           opts.workers,
		Force:             opts.force,
		RateLimitMs:       srcCfg.RateLimitMs,
		Retries:           srcCfg.EffectiveRetries(),
		RequestTimeoutSec: srcCfg.EffectiveTimeoutS(),
		MaxWaitSec:        opts.maxWaitSec,
		Headers:           srcCfg.Headers,
		Reporter:          reporter,
	})
	if err != nil {
		return nil, fmt.Errorf("creating downloader: %w", err)
	}
	return d, nil
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
