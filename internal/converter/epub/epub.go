package epub

import (
	"context"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	goepub "github.com/bmaupin/go-epub"
	"golang.org/x/image/webp"

	"github.com/ducminhgd/manga-chef/internal/converter"
)

var (
	supportedExt = map[string]struct{}{
		".jpg":  {},
		".jpeg": {},
		".png":  {},
		".webp": {},
	}
	chapterPattern = regexp.MustCompile(`(?i)^(?:chap(?:ter)?|ch)[-_ ]*([0-9]+(?:\.[0-9]+)?)$`)
)

// Converter converts chapter image folders into EPUB files.
type Converter struct{}

// New returns an EPUB converter implementation.
func New() *Converter {
	return &Converter{}
}

// Convert reads ordered images from inputDir and writes a single EPUB file.
func (c *Converter) Convert(ctx context.Context, inputDir, outputPath string, opts converter.Options) error {
	if strings.TrimSpace(inputDir) == "" {
		return errors.New("input directory is required")
	}
	if strings.TrimSpace(outputPath) == "" {
		return errors.New("output path is required")
	}

	images, err := collectImages(inputDir)
	if err != nil {
		return err
	}
	if len(images) == 0 {
		return fmt.Errorf("no supported images found in %q", inputDir)
	}

	meta := metadataFromInputPath(inputDir, opts.Title)

	book := goepub.NewEpub(meta.documentTitle)
	book.SetAuthor("manga-chef")
	book.SetDescription(fmt.Sprintf("%s %s", meta.mangaTitle, meta.chapterLabel))

	tmpFiles := make([]string, 0)
	defer func() { cleanup(tmpFiles) }()

	for i, imgPath := range images {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		sourcePath := imgPath
		internalName := filepath.Base(imgPath)
		actualExt, err := converter.DetectImageExtension(imgPath)
		if err != nil {
			return fmt.Errorf("detecting image format for %q: %w", imgPath, err)
		}

		if actualExt == ".webp" {
			sourcePath, internalName, err = webpToTempPNG(imgPath, i+1)
			if err != nil {
				return fmt.Errorf("converting webp %q: %w", imgPath, err)
			}
			tmpFiles = append(tmpFiles, sourcePath)
		} else if !strings.EqualFold(filepath.Ext(imgPath), actualExt) {
			sourcePath, internalName, err = copyToTempWithExt(imgPath, i+1, actualExt)
			if err != nil {
				return fmt.Errorf("normalizing image extension for %q: %w", imgPath, err)
			}
			tmpFiles = append(tmpFiles, sourcePath)
		}

		internalImagePath, err := book.AddImage(sourcePath, internalName)
		if err != nil {
			return fmt.Errorf("adding image %q: %w", imgPath, err)
		}
		if i == 0 {
			book.SetCover(internalImagePath, "")
		}

		body := fmt.Sprintf(`<div><img src="%s" alt="Page %d" style="max-width: 100%%; height: auto;" /></div>`, internalImagePath, i+1)
		if _, err := book.AddSection(body, fmt.Sprintf("Page %d", i+1), "", ""); err != nil {
			return fmt.Errorf("adding section for %q: %w", imgPath, err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	if err := book.Write(outputPath); err != nil {
		return fmt.Errorf("writing epub %q: %w", outputPath, err)
	}
	return nil
}

type pathMetadata struct {
	mangaTitle    string
	chapterLabel  string
	documentTitle string
}

func metadataFromInputPath(inputDir, configuredTitle string) pathMetadata {
	chapterDir := filepath.Base(filepath.Clean(inputDir))
	mangaDir := filepath.Base(filepath.Dir(filepath.Clean(inputDir)))

	manga := prettifySegment(mangaDir)
	if manga == "" || manga == "." {
		manga = "Manga"
	}
	chapter := chapterFromDir(chapterDir)

	baseTitle := strings.TrimSpace(configuredTitle)
	if baseTitle == "" {
		baseTitle = manga
	}
	docTitle := baseTitle
	if chapter != "" {
		docTitle = fmt.Sprintf("%s - %s", baseTitle, chapter)
	}

	return pathMetadata{mangaTitle: manga, chapterLabel: chapter, documentTitle: docTitle}
}

func chapterFromDir(name string) string {
	clean := strings.ToLower(strings.TrimSpace(name))
	clean = strings.ReplaceAll(clean, "_", "-")
	if m := chapterPattern.FindStringSubmatch(clean); len(m) == 2 {
		chNum := strings.TrimLeft(m[1], "0")
		if chNum == "" {
			chNum = "0"
		}
		return fmt.Sprintf("Chapter %s", chNum)
	}
	if clean == "" {
		return ""
	}
	return prettifySegment(name)
}

func prettifySegment(s string) string {
	clean := strings.TrimSpace(s)
	clean = strings.ReplaceAll(clean, "_", " ")
	clean = strings.ReplaceAll(clean, "-", " ")
	return strings.Join(strings.Fields(clean), " ")
}

func collectImages(inputDir string) ([]string, error) {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, fmt.Errorf("reading input directory %q: %w", inputDir, err)
	}

	images := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if _, ok := supportedExt[ext]; !ok {
			continue
		}
		images = append(images, filepath.Join(inputDir, entry.Name()))
	}
	sort.Strings(images)
	return images, nil
}

func webpToTempPNG(path string, idx int) (string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	img, err := webp.Decode(f)
	if err != nil {
		return "", "", err
	}

	tmp, err := os.CreateTemp("", "manga-chef-epub-*.png")
	if err != nil {
		return "", "", err
	}
	defer tmp.Close()

	if err := png.Encode(tmp, img); err != nil {
		_ = os.Remove(tmp.Name())
		return "", "", err
	}
	return tmp.Name(), fmt.Sprintf("%03d.png", idx), nil
}

func copyToTempWithExt(path string, idx int, ext string) (string, string, error) {
	tmp, err := os.CreateTemp("", "manga-chef-epub-*"+ext)
	if err != nil {
		return "", "", err
	}
	defer tmp.Close()

	in, err := os.Open(path)
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", "", err
	}
	defer in.Close()

	if _, err := io.Copy(tmp, in); err != nil {
		_ = os.Remove(tmp.Name())
		return "", "", err
	}
	return tmp.Name(), fmt.Sprintf("%03d%s", idx, ext), nil
}

func cleanup(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}
