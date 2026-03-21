package pdf

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"io"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jung-kurt/gofpdf"
	"golang.org/x/image/webp"

	"github.com/ducminhgd/manga-chef/internal/converter"
)

const (
	letterWidthInPt  = 612.0
	letterHeightInPt = 792.0
)

var supportedExt = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".webp": {},
}

// Converter converts chapter image folders into PDF files.
type Converter struct{}

// New returns a PDF converter implementation.
func New() *Converter {
	return &Converter{}
}

// Convert reads ordered images from inputDir and writes one image per PDF page.
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

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	pdf := gofpdf.New("P", "pt", "Letter", "")
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	if strings.TrimSpace(opts.Title) != "" {
		pdf.SetTitle(opts.Title, false)
	}

	tmpFiles := make([]string, 0)
	defer func() { cleanup(tmpFiles) }()

	for _, imgPath := range images {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pathToEmbed := imgPath
		actualExt, err := converter.DetectImageExtension(imgPath)
		if err != nil {
			return fmt.Errorf("detecting image format %q: %w", imgPath, err)
		}

		switch actualExt {
		case ".webp":
			pathToEmbed, err = webpToTempPNG(imgPath)
			if err != nil {
				return fmt.Errorf("converting webp %q: %w", imgPath, err)
			}
			tmpFiles = append(tmpFiles, pathToEmbed)
		default:
			if !strings.EqualFold(filepath.Ext(imgPath), actualExt) {
				pathToEmbed, err = copyToTempWithExt(imgPath, actualExt)
				if err != nil {
					return fmt.Errorf("normalizing image extension for %q: %w", imgPath, err)
				}
				tmpFiles = append(tmpFiles, pathToEmbed)
			}
		}

		cfg, err := imageConfig(pathToEmbed)
		if err != nil {
			return fmt.Errorf("decoding image %q: %w", imgPath, err)
		}

		w, h := fitWithinLetter(float64(cfg.Width), float64(cfg.Height))
		x := (letterWidthInPt - w) / 2
		y := (letterHeightInPt - h) / 2

		pdf.AddPage()
		pdf.ImageOptions(pathToEmbed, x, y, w, h, false, gofpdf.ImageOptions{ReadDpi: true}, 0, "")
	}

	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return fmt.Errorf("writing pdf %q: %w", outputPath, err)
	}
	return nil
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

func imageConfig(path string) (image.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return image.Config{}, err
	}
	return cfg, nil
}

func fitWithinLetter(imgW, imgH float64) (float64, float64) {
	if imgW <= 0 || imgH <= 0 {
		return letterWidthInPt, letterHeightInPt
	}
	scale := min(letterWidthInPt/imgW, letterHeightInPt/imgH)
	return imgW * scale, imgH * scale
}

func webpToTempPNG(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, err := webp.Decode(f)
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp("", "manga-chef-*.png")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	if err := png.Encode(tmp, img); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func copyToTempWithExt(src, ext string) (string, error) {
	tmp, err := os.CreateTemp("", "manga-chef-*"+ext)
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	in, err := os.Open(src)
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	defer in.Close()

	if _, err := io.Copy(tmp, in); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func cleanup(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}
