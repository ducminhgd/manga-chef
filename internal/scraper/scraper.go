// Package scraper defines the interface contract for manga scrapers and the
// registry that maps source codes to their factory functions.
//
// # Interface
//
// Every scraper — built-in or plugin — must implement [ScraperInterface]:
//
//	type MySource struct { ... }
//	func (s *MySource) GetChapterList(ctx context.Context, mangaURL string) ([]sources.Chapter, error) { ... }
//	func (s *MySource) GetImageURLs(ctx context.Context, chapterURL string) ([]string, error) { ... }
//
// # Registry
//
// Built-in scrapers register themselves in their package's init() function:
//
//	func init() {
//	    scraper.Register("mysource", func(cfg *sources.SourceConfig) (ScraperInterface, error) {
//	        return mysource.New(cfg), nil
//	    })
//	}
//
// The CLI resolves a scraper by calling [Get] with the source code from the
// YAML config. [Get] returns [ErrScraperNotFound] when no factory is registered
// for the requested code.
package scraper

import (
	"context"

	"github.com/ducminhgd/manga-chef/pkg/sources"
)

//go:generate mockery --name=ScraperInterface --filename=mock_scraper.go --outpkg=scraper --inpackage
//go:generate mockery --name=HTTPClient --filename=mock_http_client.go --outpkg=scraper --inpackage

// ScraperInterface is the contract every manga scraper must implement.
// It is defined here in the consumer package (scraper), not in any
// implementation package, following Go's implicit interface idiom.
//
// Both methods must:
//   - Respect context cancellation and deadline
//   - Return absolute URLs (never relative paths)
//   - Return results in display order (chapters ascending, pages ascending)
type ScraperInterface interface {
	// GetChapterList fetches the full list of chapters from a manga's main page.
	//
	// mangaURL is the absolute URL of the manga's index page on the source site
	// (e.g. "https://truyenqqno.com/truyen-tranh/dau-an-rong-thieng-236").
	//
	// Returns chapters sorted in ascending order by chapter number (chapter 1
	// first). Implementations that receive a descending list must reverse it.
	//
	// Returns an empty slice (not an error) when the manga exists but has no
	// chapters yet.
	GetChapterList(ctx context.Context, mangaURL string) ([]sources.Chapter, error)

	// GetImageURLs fetches the ordered list of image URLs for a single chapter.
	//
	// chapterURL is the absolute URL of the chapter reader page
	// (e.g. "https://truyenqqno.com/truyen-tranh/dau-an-rong-thieng-236-chap-1.html").
	//
	// Returns URLs in ascending page order (page 1 first).
	// Returns an empty slice (not an error) when the chapter has no images.
	GetImageURLs(ctx context.Context, chapterURL string) ([]string, error)
}
