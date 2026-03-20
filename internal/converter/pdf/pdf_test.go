package pdf

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ducminhgd/manga-chef/internal/converter"
)

func TestConvert_CreatesPDFFromThreeImages(t *testing.T) {
	input := t.TempDir()
	require.NoError(t, writeJPEG(filepath.Join(input, "001.jpg"), 500, 800))
	require.NoError(t, writePNG(filepath.Join(input, "002.png"), 900, 600))
	require.NoError(t, writeJPEG(filepath.Join(input, "003.jpeg"), 640, 640))

	out := filepath.Join(t.TempDir(), "chapter-001.pdf")

	conv := New()
	err := conv.Convert(context.Background(), input, out, converter.Options{Title: "Chapter 1"})
	require.NoError(t, err)

	fi, err := os.Stat(out)
	require.NoError(t, err)
	require.Greater(t, fi.Size(), int64(0))
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
			img.Set(x, y, color.RGBA{R: 32, G: 128, B: 224, A: 255})
		}
	}
	return img
}
