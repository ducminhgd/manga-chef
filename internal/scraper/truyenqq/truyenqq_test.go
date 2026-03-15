package truyenqq

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

func TestGetChapterList_ParsesTrimmedChaptersAndSortsAsc(t *testing.T) {
	html := `<!doctype html><html><body>
<div class="list_chapter"><div class="works-chapter-item"><a href="/truyen-tranh/foo-chap-10.html">Chap 10</a></div><div class="works-chapter-item"><a href="/truyen-tranh/foo-chap-1.html">Chap 1</a></div><div class="works-chapter-item"><a href="/truyen-tranh/foo-chap-2.html">Chap 2</a></div></div>
</body></html>`

	mockClient := scraper.NewMockHTTPClient(t)
	mockClient.EXPECT().Do(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(html))}, nil)

	s := &Scraper{cfg: &sources.SourceConfig{BaseURL: "https://truyenqqto.com", Headers: map[string]string{"Referer": "https://truyenqqto.com", "User-Agent": userAgentFallback}}, client: mockClient}

	chapters, err := s.GetChapterList(context.Background(), "https://truyenqqto.com/truyen-tranh/foo")
	require.NoError(t, err)
	require.Len(t, chapters, 3)
	assert.Equal(t, 1.0, chapters[0].Number)
	assert.Equal(t, "Chap 1", chapters[0].Title)
	assert.Equal(t, "https://truyenqqto.com/truyen-tranh/foo-chap-1.html", chapters[0].URL)
	assert.Equal(t, 2.0, chapters[1].Number)
	assert.Equal(t, 10.0, chapters[2].Number)
}

func TestGetImageURLs_UsesDataOriginalAndResolvesAbsoluteURLs(t *testing.T) {
	html := `<!doctype html><html><body><div class="chapter_content"><div id="page_0" class="page-chapter"><img class="lazy" src="https://i216.truyenvua.com/236/1/0.jpg" data-original="https://i216.truyenvua.com/236/1/0.jpg" data-cdn="https://hinhtruyen.com/236/1/0.jpg" /></div><div id="page_1" class="page-chapter"><img class="lazy" data-original="https://i216.truyenvua.com/236/1/1.jpg" /></div></div></body></html>`

	mockClient := scraper.NewMockHTTPClient(t)
	mockClient.EXPECT().Do(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(html))}, nil)

	s := &Scraper{cfg: &sources.SourceConfig{BaseURL: "https://truyenqqto.com", Headers: map[string]string{"Referer": "https://truyenqqto.com", "User-Agent": userAgentFallback}}, client: mockClient}

	urls, err := s.GetImageURLs(context.Background(), "https://truyenqqto.com/truyen-tranh/foo-chap-1.html")
	require.NoError(t, err)
	require.Len(t, urls, 2)
	assert.Equal(t, "https://i216.truyenvua.com/236/1/0.jpg", urls[0])
	assert.Equal(t, "https://i216.truyenvua.com/236/1/1.jpg", urls[1])
}

func TestGetImageURLs_NoImagesReturnsError(t *testing.T) {
	html := `<!doctype html><html><body><div class="chapter_content"></div></body></html>`
	mockClient := scraper.NewMockHTTPClient(t)
	mockClient.EXPECT().Do(mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(html))}, nil)

	s := &Scraper{cfg: &sources.SourceConfig{BaseURL: "https://truyenqqto.com", Headers: map[string]string{"Referer": "https://truyenqqto.com", "User-Agent": userAgentFallback}}, client: mockClient}

	_, err := s.GetImageURLs(context.Background(), "https://truyenqqto.com/truyen-tranh/foo-chap-1.html")
	require.Error(t, err)
}
