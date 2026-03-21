package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ducminhgd/manga-chef/internal/scraper"
	"github.com/ducminhgd/manga-chef/pkg/sources"
)

// Options configures the downloader behavior.
// ProgressReporter receives download progress events.
type ProgressReporter interface {
	OnStart(chapter sources.Chapter, total int)
	OnProgress(chapter sources.Chapter, done int, total int)
	OnDone(chapter sources.Chapter)
	OnError(err error)
}

// Options configures the downloader behavior.
type Options struct {
	OutDir            string
	Workers           int
	Force             bool
	RateLimitMs       int
	Retries           int
	RetryInitialMs    int
	MaxWaitSec        int
	RequestTimeoutSec int
	ImageTimeoutSec   int
	Headers           map[string]string
	Reporter          ProgressReporter
}

// Downloader downloads chapter images using a ScraperInterface.
type Downloader struct {
	scraper    scraper.ScraperInterface
	opts       Options
	httpClient *http.Client
}

// New creates a Downloader.
func New(scraper scraper.ScraperInterface, opts *Options) (*Downloader, error) {
	if scraper == nil {
		return nil, errors.New("scraper is required")
	}
	if opts == nil {
		opts = &Options{}
	}
	if opts.OutDir == "" {
		opts.OutDir = "."
	}
	if opts.Workers <= 0 {
		opts.Workers = 4
	}
	if opts.RateLimitMs < 0 {
		opts.RateLimitMs = 0
	}
	if opts.RequestTimeoutSec <= 0 {
		opts.RequestTimeoutSec = 60
	}
	if opts.ImageTimeoutSec <= 0 {
		opts.ImageTimeoutSec = 30
	}
	if opts.Retries < 0 {
		opts.Retries = 0
	}
	if opts.RetryInitialMs <= 0 {
		opts.RetryInitialMs = 300
	}
	if opts.MaxWaitSec < 0 {
		opts.MaxWaitSec = 300
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output directory %q: %w", opts.OutDir, err)
	}
	if opts.Headers == nil {
		opts.Headers = map[string]string{}
	}
	return &Downloader{scraper: scraper, opts: *opts, httpClient: &http.Client{Timeout: time.Duration(opts.RequestTimeoutSec) * time.Second}}, nil
}

// DownloadManga downloads the given chapters concurrently.
func (d *Downloader) DownloadManga(ctx context.Context, sourceCode string, chapters []sources.Chapter) error {
	if len(chapters) == 0 {
		return errors.New("no chapters to download")
	}

	sort.Slice(chapters, func(i, j int) bool { return chapters[i].Number < chapters[j].Number })

	for _, ch := range chapters {
		select {
		case <-ctx.Done():
			return wrapContextErr(ctx, "download manga")
		default:
		}
		if err := d.DownloadChapter(ctx, sourceCode, ch); err != nil {
			return fmt.Errorf("download chapter %.2f: %w", ch.Number, err)
		}
		if d.opts.RateLimitMs > 0 {
			time.Sleep(time.Duration(d.opts.RateLimitMs) * time.Millisecond)
		}
	}
	return nil
}

// DownloadChapter downloads all page images for a single chapter.
func (d *Downloader) DownloadChapter(ctx context.Context, sourceCode string, chapter sources.Chapter) error {
	urls, err := d.scraper.GetImageURLs(ctx, chapter.URL)
	if err != nil {
		return fmt.Errorf("get image urls: %w", err)
	}
	if len(urls) == 0 {
		return errors.New("no images returned by scraper")
	}

	folder := chapterFolder(d.opts.OutDir, sourceCode, chapter)
	if d.opts.Force {
		if err := os.RemoveAll(folder); err != nil {
			return fmt.Errorf("force remove existing folder: %w", err)
		}
	}

	if err := os.MkdirAll(folder, 0o755); err != nil {
		return fmt.Errorf("create folder %q: %w", folder, err)
	}

	if d.opts.Reporter != nil {
		d.opts.Reporter.OnStart(chapter, len(urls))
	}
	existing := existingFiles(folder)
	reportDone := newProgressReporter(chapter, len(urls), d.opts.Reporter)
	failCount, firstErr := d.downloadChapterImages(ctx, folder, urls, existing, reportDone)
	if d.opts.Reporter != nil {
		d.opts.Reporter.OnDone(chapter)
	}

	if failCount == len(urls) {
		if firstErr != nil {
			return fmt.Errorf("all images failed: %w", firstErr)
		}
		return errors.New("all images failed to download")
	}
	if firstErr != nil {
		return nil
	}
	return nil
}

func (d *Downloader) downloadImageWithRetry(ctx context.Context, folder string, page int, imageURL string) error {
	const attemptsPerCycle = 3

	for {
		hitRetryCycleStatus, err := d.runRetryCycle(ctx, folder, page, imageURL, attemptsPerCycle)
		if err == nil {
			return nil
		}
		if !hitRetryCycleStatus {
			return err
		}
		if err := waitForRetryCycle(ctx, time.Duration(d.opts.MaxWaitSec)*time.Second); err != nil {
			return err
		}
	}
}

func isHTTP429Error(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "status 429") || strings.Contains(err.Error(), "429") {
		return true
	}
	return false
}

func isHTTP403Error(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "status 403") || strings.Contains(err.Error(), "403") {
		return true
	}
	return false
}

func isRetryCycleStatus(err error) bool {
	return isHTTP429Error(err) || isHTTP403Error(err)
}

func (d *Downloader) downloadImage(ctx context.Context, folder string, page int, imageURL string) error {
	target := pagePath(folder, page, imageURL)
	if _, err := os.Stat(target); err == nil {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("create request for %q: %w", imageURL, err)
	}
	for k, v := range d.opts.Headers {
		req.Header.Set(k, v)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request image %q: %w", imageURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("image request status %d", resp.StatusCode)
	}

	ext := imageExtension(imageURL)
	path := filepath.Join(folder, fmt.Sprintf("%03d%s", page, ext))
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create image file %q: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write image file %q: %w", path, err)
	}
	return nil
}

func pagePath(folder string, page int, imageURL string) string {
	ext := imageExtension(imageURL)
	return filepath.Join(folder, fmt.Sprintf("%03d%s", page, ext))
}

func shouldRetryDownload(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return true
		}
		if urlErr.Err != nil {
			if strings.Contains(strings.ToLower(urlErr.Err.Error()), "connection reset") {
				return true
			}
		}
	}
	if strings.Contains(err.Error(), "status 5") {
		return true
	}
	if strings.Contains(err.Error(), "connection reset") {
		return true
	}
	if strings.Contains(err.Error(), "EOF") {
		return true
	}
	return false
}

func chapterFolder(base, sourceCode string, chapter sources.Chapter) string {
	chapNum := fmt.Sprintf("chap-%03d", int(chapter.Number))
	if chapter.Number != float64(int(chapter.Number)) {
		chapNum = fmt.Sprintf("chap-%g", chapter.Number)
	}
	return filepath.Join(base, sourceCode, chapNum)
}

func imageExtension(imageURL string) string {
	u, err := url.Parse(imageURL)
	if err == nil {
		ext := filepath.Ext(u.Path)
		if ext != "" {
			return ext
		}
	}
	return ".jpg"
}

func existingFiles(folder string) map[string]struct{} {
	existing := map[string]struct{}{}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return existing
	}
	for _, entry := range entries {
		existing[entry.Name()] = struct{}{}
	}
	return existing
}

func newProgressReporter(chapter sources.Chapter, total int, reporter ProgressReporter) func() {
	done := 0
	var mu sync.Mutex
	return func() {
		mu.Lock()
		defer mu.Unlock()
		done++
		if reporter != nil {
			reporter.OnProgress(chapter, done, total)
		}
	}
}

func (d *Downloader) downloadChapterImages(ctx context.Context, folder string, urls []string, existing map[string]struct{}, reportDone func()) (failCount int, firstErr error) {
	var (
		wg    sync.WaitGroup
		sem   = make(chan struct{}, d.opts.Workers)
		errMu sync.Mutex
	)

	for idx, imgURL := range urls {
		if err := wrapContextErr(ctx, "download chapter"); err != nil {
			return failCount, err
		}
		pagePath := pagePath(folder, idx+1, imgURL)
		if _, found := existing[filepath.Base(pagePath)]; found {
			reportDone()
			continue
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(page int, imageURL string) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := d.downloadImageWithRetry(ctx, folder, page, imageURL); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				failCount++
				errMu.Unlock()
				if d.opts.Reporter != nil {
					d.opts.Reporter.OnError(err)
				}
				return
			}
			reportDone()
		}(idx+1, imgURL)
	}

	wg.Wait()
	return failCount, firstErr
}

func (d *Downloader) runRetryCycle(ctx context.Context, folder string, page int, imageURL string, attemptsPerCycle int) (hitRetryCycleStatus bool, err error) {
	backoffMs := int64(d.opts.RetryInitialMs)
	for attempt := 1; attempt <= attemptsPerCycle; attempt++ {
		if attempt > 1 {
			if err := sleepWithContext(ctx, time.Duration(backoffMs)*time.Millisecond, "download retry backoff"); err != nil {
				return hitRetryCycleStatus, err
			}
			backoffMs *= 2
		}

		err = d.downloadImage(ctx, folder, page, imageURL)
		if err == nil {
			return hitRetryCycleStatus, nil
		}
		if isRetryCycleStatus(err) {
			hitRetryCycleStatus = true
			continue
		}
		if !shouldRetryDownload(err) {
			return hitRetryCycleStatus, fmt.Errorf("download image %q failed permanently: %w", imageURL, err)
		}
		if attempt == attemptsPerCycle {
			return hitRetryCycleStatus, fmt.Errorf("download image %q failed after %d attempts: %w", imageURL, attemptsPerCycle, err)
		}
	}

	return hitRetryCycleStatus, fmt.Errorf("download image %q failed after %d attempts", imageURL, attemptsPerCycle)
}

func waitForRetryCycle(ctx context.Context, waitFor time.Duration) error {
	if waitFor > 0 {
		if err := sleepWithContext(ctx, waitFor, "waiting before retry cycle"); err != nil {
			return err
		}
	}
	return wrapContextErr(ctx, "waiting before retry cycle")
}

func sleepWithContext(ctx context.Context, duration time.Duration, action string) error {
	select {
	case <-ctx.Done():
		return wrapContextErr(ctx, action)
	case <-time.After(duration):
		return nil
	}
}

func wrapContextErr(ctx context.Context, action string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}
