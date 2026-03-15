package generic

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ducminhgd/manga-chef/internal/scraper"
	"github.com/ducminhgd/manga-chef/pkg/sources"
)

func TestGetChapterList_ParsesChaptersFromSelector(t *testing.T) {
	html := `<!doctype html><html><body><ul class="chapters"><li><a href="/manga/chapter-1.html">Chap 1</a></li><li><a href="/manga/chapter-2.html">Chap 2</a></li></ul></body></html>`

	mockClient := scraper.NewMockHTTPClient(t)
	mockClient.EXPECT().Do(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(html))}, nil)

	s := &Scraper{cfg: &sources.SourceConfig{BaseURL: "https://example.com", Selectors: &sources.Selectors{ChapterList: "ul.chapters li a", ImageList: "div img", ImageURL: "attr:src"}}, client: mockClient}

	chapters, err := s.GetChapterList(context.Background(), "https://example.com/manga")
	require.NoError(t, err)
	require.Len(t, chapters, 2)
	assert.Equal(t, 1.0, chapters[0].Number)
	assert.Equal(t, "Chap 1", chapters[0].Title)
	assert.Equal(t, "https://example.com/manga/chapter-1.html", chapters[0].URL)
}

func TestGetImageURLs_UsesConfiguredImageURLSelector(t *testing.T) {
	html := `<!doctype html><html><body><div class="page"><img data-src="/img1.jpg"/><img data-src="/img2.jpg"/></div></body></html>`

	mockClient := scraper.NewMockHTTPClient(t)
	mockClient.EXPECT().Do(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(html))}, nil)

	s := &Scraper{cfg: &sources.SourceConfig{BaseURL: "https://example.com", Selectors: &sources.Selectors{ChapterList: "a", ImageList: "div.page img", ImageURL: "attr:data-src"}}, client: mockClient}

	urls, err := s.GetImageURLs(context.Background(), "https://example.com/manga/chapter-1")
	require.NoError(t, err)
	require.Len(t, urls, 2)
	assert.Equal(t, "https://example.com/img1.jpg", urls[0])
	assert.Equal(t, "https://example.com/img2.jpg", urls[1])
}

func TestGetImageURLs_ReturnsErrorWhenNoImages(t *testing.T) {
	html := `<!doctype html><html><body><div class="page"></div></body></html>`
	mockClient := scraper.NewMockHTTPClient(t)
	mockClient.EXPECT().Do(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(html))}, nil)

	s := &Scraper{cfg: &sources.SourceConfig{BaseURL: "https://example.com", Selectors: &sources.Selectors{ChapterList: "a", ImageList: "div.page img", ImageURL: "attr:data-src"}}, client: mockClient}

	_, err := s.GetImageURLs(context.Background(), "https://example.com/manga/chapter-1")
	require.Error(t, err)
}
