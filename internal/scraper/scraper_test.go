package scraper_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ducminhgd/manga-chef/internal/scraper"
	"github.com/ducminhgd/manga-chef/pkg/sources"
)

// ── Compile-time interface satisfaction checks ────────────────────────────────
// These blank assignments fail to compile if the types no longer satisfy the
// interfaces, catching interface drift before any test runs.

var _ scraper.ScraperInterface = (*scraper.MockScraper)(nil)
var _ scraper.HTTPClient = (*scraper.MockHTTPClient)(nil)

// ── MockScraper behaviour ─────────────────────────────────────────────────────

func TestMockScraper_GetChapterList_ReturnsConfiguredValue(t *testing.T) {
	m := scraper.NewMockScraper(t)
	ctx := context.Background()
	mangaURL := "https://truyenqqto.com/truyen-tranh/dau-an-rong-thieng-236"

	want := []sources.Chapter{
		{Number: 1, Title: "Chapter 1", URL: "https://truyenqqto.com/.../chap-1.html"},
		{Number: 2, Title: "Chapter 2", URL: "https://truyenqqto.com/.../chap-2.html"},
	}

	m.EXPECT().GetChapterList(ctx, mangaURL).Return(want, nil)

	got, err := m.GetChapterList(ctx, mangaURL)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMockScraper_GetChapterList_ReturnsError(t *testing.T) {
	m := scraper.NewMockScraper(t)
	ctx := context.Background()

	m.EXPECT().
		GetChapterList(ctx, "https://example.com/manga").
		Return(nil, assert.AnError)

	_, err := m.GetChapterList(ctx, "https://example.com/manga")
	require.Error(t, err)
}

func TestMockScraper_GetImageURLs_ReturnsConfiguredValue(t *testing.T) {
	m := scraper.NewMockScraper(t)
	ctx := context.Background()
	chapterURL := "https://truyenqqto.com/truyen-tranh/dau-an-rong-thieng-236-chap-1.html"

	want := []string{
		"https://cdn.truyenqqto.com/chap1/page001.jpg",
		"https://cdn.truyenqqto.com/chap1/page002.jpg",
		"https://cdn.truyenqqto.com/chap1/page003.jpg",
	}

	m.EXPECT().GetImageURLs(ctx, chapterURL).Return(want, nil)

	got, err := m.GetImageURLs(ctx, chapterURL)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Len(t, got, 3)
}

func TestMockScraper_GetImageURLs_EmptyChapter(t *testing.T) {
	m := scraper.NewMockScraper(t)
	ctx := context.Background()

	m.EXPECT().GetImageURLs(ctx, "https://example.com/ch0").Return([]string{}, nil)

	got, err := m.GetImageURLs(ctx, "https://example.com/ch0")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestMockScraper_UnmetExpectation_FailsTest(t *testing.T) {
	// Verify t.Cleanup fires AssertExpectations correctly by using a fake T
	// so the outer test does not fail.
	t.Run("unmet expectation is caught", func(t *testing.T) {
		fakeT := &fakeTestingT{}
		m := scraper.NewMockScraper(fakeT)
		m.EXPECT().
			GetChapterList(context.Background(), "https://x.com").
			Return(nil, nil)

		// Deliberately NOT calling GetChapterList — cleanup should catch it.
		fakeT.runCleanup()

		assert.True(t, fakeT.failed,
			"AssertExpectations should mark the test as failed when an expectation was not met")
	})
}

// ── MockHTTPClient behaviour ─────────────────────────────────────────────────

func TestMockHTTPClient_Do_ReturnsConfiguredResponse(t *testing.T) {
	m := scraper.NewMockHTTPClient(t)
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "https://example.com", http.NoBody)
	require.NoError(t, err)

	want := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("hello")),
	}
	m.EXPECT().Do(req).Return(want, nil)

	got, err := m.Do(req)
	require.NoError(t, err)
	defer got.Body.Close()
	assert.Equal(t, http.StatusOK, got.StatusCode)
}

func TestMockHTTPClient_Do_ReturnsError(t *testing.T) {
	m := scraper.NewMockHTTPClient(t)
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "https://example.com", http.NoBody)
	require.NoError(t, err)

	m.EXPECT().Do(req).Return(nil, assert.AnError)

	got, err := m.Do(req)
	require.Error(t, err)
	if got != nil && got.Body != nil {
		_ = got.Body.Close()
	}
}

// ── fakeTestingT ─────────────────────────────────────────────────────────────
// fakeTestingT lets us assert that cleanup functions call Errorf on an
// unmet expectation without failing the real test.

type fakeTestingT struct {
	failed   bool
	cleanups []func()
}

func (f *fakeTestingT) Cleanup(fn func())                 { f.cleanups = append(f.cleanups, fn) }
func (f *fakeTestingT) FailNow()                          { f.failed = true }
func (f *fakeTestingT) Errorf(_ string, _ ...interface{}) { f.failed = true }
func (f *fakeTestingT) Logf(_ string, _ ...interface{})   {}
func (f *fakeTestingT) runCleanup() {
	for _, fn := range f.cleanups {
		fn()
	}
}
