package truyenqq

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
	"time"

	"golang.org/x/net/html"

	"github.com/ducminhgd/manga-chef/internal/scraper"
	"github.com/ducminhgd/manga-chef/pkg/sources"
)

var chapterNumRegex = regexp.MustCompile(`(?i)(?:chap(?:ter)?|ch(?:ương)?)[^0-9]*([0-9]+(?:\.[0-9]+)?)`)
var chapterNumInURLRegex = regexp.MustCompile(`(?i)chap(?:ter)?[-_]?(\d+(?:\.\d+)?)`)

const userAgentFallback = "Mozilla/5.0 (compatible; manga-chef/1.0; +https://github.com/ducminhgd/manga-chef)"

// Scraper implements scraper.ScraperInterface for TruyenQQ.
type Scraper struct {
	cfg    *sources.SourceConfig
	client scraper.HTTPClient
}

// New constructs a TruyenQQ Scraper.
func New(cfg *sources.SourceConfig) *Scraper {
	if cfg == nil {
		cfg = &sources.SourceConfig{}
	}
	client := scraper.NewHTTPClient(cfg)
	return &Scraper{cfg: cfg, client: client}
}

// Register registers the TruyenQQ scraper into the global registry.
func Register() {
	scraper.Register("truyenqq", func(cfg *sources.SourceConfig) (scraper.ScraperInterface, error) {
		if cfg == nil {
			return nil, errors.New("source config is required")
		}
		return New(cfg), nil
	})
}

func init() {
	Register()
}

// GetChapterList fetches and parses the chapter list from a manga main page.
func (s *Scraper) GetChapterList(ctx context.Context, mangaURL string) ([]sources.Chapter, error) {
	doc, err := s.fetchHTML(ctx, mangaURL)
	if err != nil {
		return nil, err
	}

	anchors := s.collectChapterAnchors(doc, mangaURL)
	chapters := make([]sources.Chapter, 0, len(anchors))
	seen := make(map[string]struct{})
	for _, a := range anchors {
		href := getAttr(a, "href")
		if href == "" {
			continue
		}

		abs, err := resolveURL(mangaURL, href)
		if err != nil {
			continue
		}
		if _, dup := seen[abs]; dup {
			continue
		}
		seen[abs] = struct{}{}

		number, ok := parseChapterNumber(textContent(a))
		if !ok {
			number, ok = parseChapterNumberFromURL(abs)
		}
		if !ok {
			continue
		}

		title := strings.TrimSpace(textContent(a))
		if title == "" {
			title = fmt.Sprintf("Chap %.1f", number)
		}

		chapters = append(chapters, sources.Chapter{
			Number: number,
			Title:  title,
			URL:    abs,
		})
	}

	sort.SliceStable(chapters, func(i, j int) bool {
		if chapters[i].Number != chapters[j].Number {
			return chapters[i].Number < chapters[j].Number
		}
		return chapters[i].URL < chapters[j].URL
	})

	return chapters, nil
}

// GetImageURLs fetches and parses image URLs from a chapter page.
func (s *Scraper) GetImageURLs(ctx context.Context, chapterURL string) ([]string, error) {
	doc, err := s.fetchHTML(ctx, chapterURL)
	if err != nil {
		return nil, err
	}

	imgs := collectImageNodes(doc)
	if len(imgs) == 0 {
		return nil, fmt.Errorf("no chapter images found")
	}

	urls := make([]string, 0, len(imgs))
	for _, img := range imgs {
		imgURL := getAttr(img, "data-original")
		if imgURL == "" {
			imgURL = getAttr(img, "data-src")
		}
		if imgURL == "" {
			imgURL = getAttr(img, "data-cdn")
		}
		if imgURL == "" {
			imgURL = getAttr(img, "src")
		}
		if imgURL == "" {
			continue
		}

		abs, err := resolveURL(chapterURL, imgURL)
		if err != nil {
			continue
		}
		urls = append(urls, abs)
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("no valid image URLs found")
	}
	return urls, nil
}

func (s *Scraper) fetchHTML(ctx context.Context, targetURL string) (*html.Node, error) {
	backoffMs := int64(500)
	for attempt := 0; attempt < 4; attempt++ {
		doc, retry, err := s.fetchHTMLAttempt(ctx, targetURL, attempt)
		if err == nil && !retry {
			return doc, nil
		}
		if !retry {
			return nil, err
		}
		if err := waitForHTMLRetry(ctx, time.Duration(backoffMs)*time.Millisecond); err != nil {
			return nil, err
		}
		backoffMs *= 2
	}
	return nil, fmt.Errorf("http GET %s: retry attempts exhausted", targetURL)
}

func (s *Scraper) fetchHTMLAttempt(ctx context.Context, targetURL string, attempt int) (doc *html.Node, retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, http.NoBody)
	if err != nil {
		return nil, false, fmt.Errorf("request new: %w", err)
	}

	for name, value := range s.cfg.Headers {
		req.Header.Set(name, value)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgentFallback)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("http GET %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		doc, err = html.Parse(resp.Body)
		if err != nil {
			return nil, false, fmt.Errorf("parsing html: %w", err)
		}
		return doc, false, nil
	case http.StatusTooManyRequests:
		if attempt == 3 {
			return nil, false, readHTTPStatusError(targetURL, resp)
		}
		return nil, true, nil
	default:
		return nil, false, readHTTPStatusError(targetURL, resp)
	}
}

func (s *Scraper) collectChapterAnchors(doc *html.Node, baseURL string) []*html.Node {
	var container *html.Node = findFirstNode(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		cls := getAttr(n, "class")
		return strings.Contains(cls, "list_chapter") || strings.Contains(cls, "list-chapter") || strings.Contains(cls, "works-chapter-list")
	})

	var candidates []*html.Node
	if container != nil {
		candidates = collectAnchors(container)
	} else {
		candidates = collectAnchors(doc)
	}

	filtered := make([]*html.Node, 0, len(candidates))
	for _, a := range candidates {
		href := getAttr(a, "href")
		if href == "" {
			continue
		}
		if !strings.Contains(strings.ToLower(href), "chap") {
			continue
		}
		abs, err := resolveURL(baseURL, href)
		if err != nil {
			continue
		}
		if !strings.Contains(strings.ToLower(abs), "chap") {
			continue
		}
		filtered = append(filtered, a)
	}
	return filtered
}

func collectAnchors(root *html.Node) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func collectImageNodes(doc *html.Node) []*html.Node {
	var out []*html.Node
	var walkContainer func(*html.Node)
	walkContainer = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			if getAttr(n, "data-original") != "" || getAttr(n, "data-src") != "" || getAttr(n, "data-cdn") != "" || getAttr(n, "src") != "" {
				out = append(out, n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkContainer(c)
		}
	}

	pageContainers := findNodes(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "div" {
			return false
		}
		cls := getAttr(n, "class")
		return strings.Contains(cls, "page-chapter")
	})

	for _, c := range pageContainers {
		walkContainer(c)
	}
	return out
}

func findNodes(root *html.Node, pred func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if pred(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func resolveURL(base, ref string) (string, error) {
	parsedBase, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse base url %q: %w", base, err)
	}
	rel, err := url.Parse(ref)
	if err != nil {
		return "", fmt.Errorf("parse relative url %q: %w", ref, err)
	}
	return parsedBase.ResolveReference(rel).String(), nil
}

func readHTTPStatusError(targetURL string, resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return fmt.Errorf("reading error response body: %w", err)
	}
	return fmt.Errorf("http GET %s: status %d, body: %s", targetURL, resp.StatusCode, strings.TrimSpace(string(body)))
}

func waitForHTMLRetry(ctx context.Context, backoff time.Duration) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("retrying html fetch: %w", ctx.Err())
	case <-time.After(backoff):
		return nil
	}
}

func parseChapterNumber(text string) (float64, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false
	}
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

func getAttr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, name) {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

func findFirstNode(root *html.Node, pred func(*html.Node) bool) *html.Node {
	if root == nil {
		return nil
	}
	if pred(root) {
		return root
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirstNode(c, pred); found != nil {
			return found
		}
	}
	return nil
}

func textContent(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(c *html.Node) {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
		for k := c.FirstChild; k != nil; k = k.NextSibling {
			walk(k)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}
