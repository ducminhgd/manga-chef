package epub

import (
	"archive/zip"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ducminhgd/manga-chef/internal/converter"
)

func TestConvert_CreatesEpubFromThreeImages(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "Dragon-Quest", "chap-001")
	require.NoError(t, os.MkdirAll(input, 0o755))
	require.NoError(t, writeJPEG(filepath.Join(input, "001.jpg"), 500, 800))
	require.NoError(t, writePNG(filepath.Join(input, "002.png"), 900, 600))
	require.NoError(t, writeJPEG(filepath.Join(input, "003.jpeg"), 640, 640))

	out := filepath.Join(t.TempDir(), "chapter-001.epub")

	conv := New()
	err := conv.Convert(context.Background(), input, out, converter.Options{})
	require.NoError(t, err)

	fi, err := os.Stat(out)
	require.NoError(t, err)
	require.Greater(t, fi.Size(), int64(0))

	zr, err := zip.OpenReader(out)
	require.NoError(t, err)
	defer zr.Close()

	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	require.Contains(t, names, "mimetype")
	require.Contains(t, names, "EPUB/package.opf")

	opf := readZipFile(t, zr.File, "EPUB/package.opf")
	require.Contains(t, opf, "Dragon Quest - Chapter 1")
	require.Contains(t, opf, "cover-image")
}

func readZipFile(t *testing.T, files []*zip.File, name string) string {
	t.Helper()
	for _, f := range files {
		if f.Name != name {
			continue
		}
		r, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(r)
		require.NoError(t, r.Close())
		require.NoError(t, err)
		return string(data)
	}
	t.Fatalf("file %q not found in zip", name)
	return ""
}

func writeJPEG(path string, w, h int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	img := solidImage(w, h)
	return jpeg.Encode(f, img, &jpeg.Options{Quality: 85})
}

func writePNG(path string, w, h int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	img := solidImage(w, h)
	return png.Encode(f, img)
}

func solidImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 40, G: 190, B: 120, A: 255})
		}
	}
	return img
}
