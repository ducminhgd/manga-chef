package generic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/ducminhgd/manga-chef/internal/scraper"
	"github.com/ducminhgd/manga-chef/pkg/sources"
)

var chapterNumRegex = regexp.MustCompile(`(?i)(?:chap(?:ter)?|ch(?:ương)?)[^0-9]*([0-9]+(?:\.[0-9]+)?)`)
var chapterNumInURLRegex = regexp.MustCompile(`(?i)chap(?:ter)?[-_]?([0-9]+(?:\.[0-9]+)?)`)

const userAgentFallback = "Mozilla/5.0 (compatible; manga-chef/1.0; +https://github.com/ducminhgd/manga-chef)"

// Scraper implements scraper.ScraperInterface for generic HTML sources.
type Scraper struct {
	cfg    *sources.SourceConfig
	client scraper.HTTPClient
}

// New constructs a generic scraper instance.
func New(cfg *sources.SourceConfig) *Scraper {
	if cfg == nil {
		cfg = &sources.SourceConfig{}
	}
	client := scraper.NewHTTPClient(cfg)
	return &Scraper{cfg: cfg, client: client}
}

// Register registers the generic scraper in the global registry.
func Register() {
	scraper.Register("generic", func(cfg *sources.SourceConfig) (scraper.ScraperInterface, error) {
		if cfg == nil {
			return nil, errors.New("source config is required")
		}
		if cfg.Selectors == nil {
			return nil, errors.New("generic scraper requires selectors")
		}
		if strings.TrimSpace(cfg.Selectors.ChapterList) == "" {
			return nil, errors.New("generic scraper selectors.chapter_list is required")
		}
		if strings.TrimSpace(cfg.Selectors.ImageList) == "" {
			return nil, errors.New("generic scraper selectors.image_list is required")
		}
		if strings.TrimSpace(cfg.Selectors.ImageURL) == "" {
			return nil, errors.New("generic scraper selectors.image_url is required")
		}
		return New(cfg), nil
	})
}

func init() {
	Register()
}

// GetChapterList fetches and parses chapters using configured CSS selectors.
func (s *Scraper) GetChapterList(ctx context.Context, mangaURL string) ([]sources.Chapter, error) {
	doc, err := s.fetchHTML(ctx, mangaURL)
	if err != nil {
		return nil, err
	}

	if s.cfg == nil || s.cfg.Selectors == nil {
		return nil, errors.New("generic scraper requires selectors")
	}

	chapterSel := strings.TrimSpace(s.cfg.Selectors.ChapterList)
	if chapterSel == "" {
		return nil, errors.New("generic scraper selectors.chapter_list is required")
	}

	selection := doc.Find(chapterSel)
	chapters := make([]sources.Chapter, 0, selection.Length())
	seen := make(map[string]struct{})

	selection.Each(func(i int, sel *goquery.Selection) {
		anchor := sel
		if goquery.NodeName(sel) != "a" {
			a := sel.Find("a").First()
			if a.Length() > 0 {
				anchor = a
			}
		}

		href, ok := anchor.Attr("href")
		if !ok || strings.TrimSpace(href) == "" {
			return
		}

		abs, err := resolveURL(mangaURL, href)
		if err != nil {
			return
		}
		if _, dup := seen[abs]; dup {
			return
		}
		seen[abs] = struct{}{}

		title := strings.TrimSpace(anchor.Text())
		number, ok := s.parseChapterNumber(sel, anchor)
		if !ok {
			number, ok = parseChapterNumberFromURL(abs)
		}
		if !ok {
			return
		}

		if title == "" {
			title = fmt.Sprintf("Chap %.1f", number)
		}

		chapters = append(chapters, sources.Chapter{Number: number, Title: title, URL: abs})
	})

	sort.SliceStable(chapters, func(i, j int) bool {
		if chapters[i].Number != chapters[j].Number {
			return chapters[i].Number < chapters[j].Number
		}
		return chapters[i].URL < chapters[j].URL
	})

	return chapters, nil
}

// GetImageURLs fetches image URLs from a chapter page using configured selectors.
func (s *Scraper) GetImageURLs(ctx context.Context, chapterURL string) ([]string, error) {
	doc, err := s.fetchHTML(ctx, chapterURL)
	if err != nil {
		return nil, err
	}

	if s.cfg == nil || s.cfg.Selectors == nil {
		return nil, errors.New("generic scraper requires selectors")
	}

	imageList := strings.TrimSpace(s.cfg.Selectors.ImageList)
	if imageList == "" {
		return nil, errors.New("generic scraper selectors.image_list is required")
	}

	imageURLSel := strings.TrimSpace(s.cfg.Selectors.ImageURL)
	if imageURLSel == "" {
		return nil, errors.New("generic scraper selectors.image_url is required")
	}

	selection := doc.Find(imageList)
	if selection.Length() == 0 {
		return nil, fmt.Errorf("no image nodes found using selector %q", imageList)
	}

	urls := make([]string, 0, selection.Length())
	selection.Each(func(i int, sel *goquery.Selection) {
		imgURL := valueForSelector(sel, imageURLSel)
		if imgURL == "" {
			return
		}
		abs, err := resolveURL(chapterURL, imgURL)
		if err != nil {
			return
		}
		urls = append(urls, abs)
	})

	if len(urls) == 0 {
		return nil, errors.New("no valid image URLs found")
	}
	return urls, nil
}

func valueForSelector(sel *goquery.Selection, selector string) string {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return ""
	}
	if strings.HasPrefix(selector, "attr:") {
		attrName := strings.TrimSpace(strings.TrimPrefix(selector, "attr:"))
		if attrName == "" {
			return ""
		}
		if v, ok := sel.Attr(attrName); ok {
			return strings.TrimSpace(v)
		}
		img := sel.Find("img").First()
		if img.Length() > 0 {
			if v, ok := img.Attr(attrName); ok {
				return strings.TrimSpace(v)
			}
		}
		return ""
	}
	if sub := sel.Find(selector).First(); sub.Length() > 0 {
		return strings.TrimSpace(sub.Text())
	}
	if nodeName := goquery.NodeName(sel); nodeName != "" {
		if goquery.NodeName(sel) == selector {
			return strings.TrimSpace(sel.Text())
		}
	}
	return strings.TrimSpace(sel.Text())
}

func (s *Scraper) parseChapterNumber(item, anchor *goquery.Selection) (float64, bool) {
	sel := strings.TrimSpace(s.cfg.Selectors.ChapterNumber)
	if sel == "" {
		num, ok := parseChapterNumber(strings.TrimSpace(anchor.Text()))
		if ok {
			return num, true
		}
		if href, ok := anchor.Attr("href"); ok {
			return parseChapterNumberFromURL(href)
		}
		return 0, false
	}

	value := valueForSelector(item, sel)
	if value == "" {
		// fallback to anchor text
		value = strings.TrimSpace(anchor.Text())
	}
	num, ok := parseChapterNumber(value)
	if ok {
		return num, true
	}
	if href, ok := anchor.Attr("href"); ok {
		return parseChapterNumberFromURL(href)
	}
	return 0, false
}

func parseChapterNumber(text string) (float64, bool) {
	m := chapterNumRegex.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseChapterNumberFromURL(urlStr string) (float64, bool) {
	m := chapterNumInURLRegex.FindStringSubmatch(urlStr)
	if len(m) < 2 {
		return 0, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func (s *Scraper) fetchHTML(ctx context.Context, targetURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("request new: %w", err)
	}

	for name, value := range s.cfg.Headers {
		req.Header.Set(name, value)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgentFallback)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http GET %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("http GET %s: status %d, body: %s", targetURL, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing html: %w", err)
	}
	return doc, nil
}

func resolveURL(base, ref string) (string, error) {
	parsedBase, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	rel, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return parsedBase.ResolveReference(rel).String(), nil
}
