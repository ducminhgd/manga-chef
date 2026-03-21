package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ducminhgd/manga-chef/pkg/sources"
)

type fakeScraper struct {
	chapters []sources.Chapter
	images   map[string][]string
}

func (f *fakeScraper) GetChapterList(context.Context, string) ([]sources.Chapter, error) {
	return f.chapters, nil
}

func (f *fakeScraper) GetImageURLs(context.Context, string) ([]string, error) {
	return f.images["url"], nil
}

func TestDownloadChapter_CreatesFiles(t *testing.T) {
	tmp := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("image"))
		require.NoError(t, err)
	}))
	defer srv.Close()

	d, err := New(&fakeScraper{images: map[string][]string{"url": {srv.URL + "/1.jpg", srv.URL + "/2.jpg"}}}, &Options{OutDir: tmp, Workers: 2})
	require.NoError(t, err)

	err = d.DownloadChapter(context.Background(), "mycode", sources.Chapter{Number: 1, Title: "ch1", URL: "url"})
	require.NoError(t, err)

	files, err := os.ReadDir(filepath.Join(tmp, "mycode", "chap-001"))
	require.NoError(t, err)
	require.Len(t, files, 2)
}

func TestDownloadChapter_RetriesTransientFailures(t *testing.T) {
	tmp := t.TempDir()
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("image"))
		require.NoError(t, err)
	}))
	defer srv.Close()

	d, err := New(&fakeScraper{images: map[string][]string{"url": {srv.URL + "/1.jpg"}}}, &Options{OutDir: tmp, Retries: 2, RetryInitialMs: 1, Workers: 1})
	require.NoError(t, err)

	err = d.DownloadChapter(context.Background(), "mycode", sources.Chapter{Number: 1, Title: "ch1", URL: "url"})
	require.NoError(t, err)
	require.GreaterOrEqual(t, callCount, 2)

	files, err := os.ReadDir(filepath.Join(tmp, "mycode", "chap-001"))
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func TestDownloadChapter_DoesNotRetryClientErrors(t *testing.T) {
	tmp := t.TempDir()
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	d, err := New(&fakeScraper{images: map[string][]string{"url": {srv.URL + "/1.jpg"}}}, &Options{OutDir: tmp, Retries: 3, RetryInitialMs: 1, Workers: 1})
	require.NoError(t, err)

	err = d.DownloadChapter(context.Background(), "mycode", sources.Chapter{Number: 1, Title: "ch1", URL: "url"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "status 404")
	require.Equal(t, 1, callCount)
}

func TestDownloadChapter_ProgressReporter(t *testing.T) {
	tmp := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("image"))
		require.NoError(t, err)
	}))
	defer srv.Close()

	reporter := &fakeProgressReporter{}
	d, err := New(&fakeScraper{images: map[string][]string{"url": {srv.URL + "/1.jpg", srv.URL + "/2.jpg"}}}, &Options{OutDir: tmp, Workers: 2, Reporter: reporter})
	require.NoError(t, err)

	err = d.DownloadChapter(context.Background(), "mycode", sources.Chapter{Number: 1, Title: "ch1", URL: "url"})
	require.NoError(t, err)
	require.Equal(t, 1, reporter.starts)
	require.Equal(t, 2, reporter.progressEvents)
	require.Equal(t, 1, reporter.done)
}

func TestDownloadChapter_429RetryCyclesAfterThreeAttempts(t *testing.T) {
	tmp := t.TempDir()
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("image"))
		require.NoError(t, err)
	}))
	defer srv.Close()

	d, err := New(&fakeScraper{images: map[string][]string{"url": {srv.URL + "/1.jpg"}}}, &Options{
		OutDir:         tmp,
		RetryInitialMs: 1,
		MaxWaitSec:     0, // keep test fast; still exercises cycle reset
		Workers:        1,
	})
	require.NoError(t, err)

	err = d.DownloadChapter(context.Background(), "mycode", sources.Chapter{Number: 1, Title: "ch1", URL: "url"})
	require.NoError(t, err)
	require.GreaterOrEqual(t, callCount, 4)
}

func TestDownloadChapter_403RetryCyclesAfterThreeAttempts(t *testing.T) {
	tmp := t.TempDir()
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 3 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("image"))
		require.NoError(t, err)
	}))
	defer srv.Close()

	d, err := New(&fakeScraper{images: map[string][]string{"url": {srv.URL + "/1.jpg"}}}, &Options{
		OutDir:         tmp,
		RetryInitialMs: 1,
		MaxWaitSec:     0, // keep test fast; still exercises cycle reset
		Workers:        1,
	})
	require.NoError(t, err)

	err = d.DownloadChapter(context.Background(), "mycode", sources.Chapter{Number: 1, Title: "ch1", URL: "url"})
	require.NoError(t, err)
	require.GreaterOrEqual(t, callCount, 4)
}

type fakeProgressReporter struct {
	starts         int
	progressEvents int
	done           int
	err            int
}

func (f *fakeProgressReporter) OnStart(chapter sources.Chapter, total int) {
	f.starts++
}
func (f *fakeProgressReporter) OnProgress(chapter sources.Chapter, done, total int) {
	f.progressEvents++
}
func (f *fakeProgressReporter) OnDone(chapter sources.Chapter) {
	f.done++
}
func (f *fakeProgressReporter) OnError(err error) {
	f.err++
}
