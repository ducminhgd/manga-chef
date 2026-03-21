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
	chapterPattern = regexp.MustCompile(`(?i)^(?:chap(?:ter)?|ch)[-_ ]*(\d+(?:\.\d+)?)$`)
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
		if err := wrapContextErr(ctx, "converting epub"); err != nil {
			return err
		}

		sourcePath, internalName, cleanupPath, err := prepareImage(imgPath, i+1)
		if err != nil {
			return err
		}
		if cleanupPath != "" {
			tmpFiles = append(tmpFiles, cleanupPath)
		}

		internalImagePath, err := book.AddImage(sourcePath, internalName)
		if err != nil {
			return fmt.Errorf("adding image %q: %w", imgPath, err)
		}
		if i == 0 {
			book.SetCover(internalImagePath, "")
		}

		body := fmt.Sprintf(`<div><img src=%q alt="Page %d" style="max-width: 100%%; height: auto;" /></div>`, internalImagePath, i+1)
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

func prepareImage(imgPath string, idx int) (sourcePath, internalName, cleanupPath string, err error) {
	sourcePath = imgPath
	internalName = filepath.Base(imgPath)

	actualExt, err := converter.DetectImageExtension(imgPath)
	if err != nil {
		return "", "", "", fmt.Errorf("detecting image format for %q: %w", imgPath, err)
	}

	switch {
	case actualExt == ".webp":
		sourcePath, internalName, err = webpToTempPNG(imgPath, idx)
		if err != nil {
			return "", "", "", fmt.Errorf("converting webp %q: %w", imgPath, err)
		}
		return sourcePath, internalName, sourcePath, nil
	case strings.EqualFold(filepath.Ext(imgPath), actualExt):
		return sourcePath, internalName, "", nil
	default:
		sourcePath, internalName, err = copyToTempWithExt(imgPath, idx, actualExt)
		if err != nil {
			return "", "", "", fmt.Errorf("normalizing image extension for %q: %w", imgPath, err)
		}
		return sourcePath, internalName, sourcePath, nil
	}
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

func webpToTempPNG(path string, idx int) (tmpPath, internalName string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("opening webp %q: %w", path, err)
	}
	defer f.Close()

	img, err := webp.Decode(f)
	if err != nil {
		return "", "", fmt.Errorf("decoding webp %q: %w", path, err)
	}

	tmp, err := os.CreateTemp("", "manga-chef-epub-*.png")
	if err != nil {
		return "", "", fmt.Errorf("creating temp png for %q: %w", path, err)
	}
	defer tmp.Close()

	if err := png.Encode(tmp, img); err != nil {
		_ = os.Remove(tmp.Name())
		return "", "", fmt.Errorf("encoding png for %q: %w", path, err)
	}
	return tmp.Name(), fmt.Sprintf("%03d.png", idx), nil
}

func copyToTempWithExt(path string, idx int, ext string) (tmpPath, internalName string, err error) {
	tmp, err := os.CreateTemp("", "manga-chef-epub-*"+ext)
	if err != nil {
		return "", "", fmt.Errorf("creating temp file for %q: %w", path, err)
	}
	defer tmp.Close()

	in, err := os.Open(path)
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", "", fmt.Errorf("opening %q: %w", path, err)
	}
	defer in.Close()

	if _, err := io.Copy(tmp, in); err != nil {
		_ = os.Remove(tmp.Name())
		return "", "", fmt.Errorf("copying %q into temp file: %w", path, err)
	}
	return tmp.Name(), fmt.Sprintf("%03d%s", idx, ext), nil
}

func cleanup(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}

func wrapContextErr(ctx context.Context, action string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}
