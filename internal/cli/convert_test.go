package cli_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ducminhgd/manga-chef/internal/cli"
)

func TestConvertCommand_Success(t *testing.T) {
	input := t.TempDir()
	writeJPEG(t, filepath.Join(input, "001.jpg"), 320, 480)
	writeJPEG(t, filepath.Join(input, "002.jpg"), 640, 480)
	writeJPEG(t, filepath.Join(input, "003.jpg"), 480, 640)

	output := filepath.Join(t.TempDir(), "chapter.pdf")

	var buf bytes.Buffer
	root := cli.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"convert", "--input", input, "--format", "pdf", "--output", output})

	err := root.Execute()
	require.NoError(t, err)

	fi, err := os.Stat(output)
	require.NoError(t, err)
	require.Greater(t, fi.Size(), int64(0))
}

func TestConvertCommand_RequiresOutput(t *testing.T) {
	input := t.TempDir()
	writeJPEG(t, filepath.Join(input, "001.jpg"), 300, 300)

	var buf bytes.Buffer
	root := cli.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"convert", "--input", input, "--format", "pdf"})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--output is required")
}

func TestConvertCommand_SupportsEpub(t *testing.T) {
	input := t.TempDir()
	writeJPEG(t, filepath.Join(input, "001.jpg"), 320, 480)
	writeJPEG(t, filepath.Join(input, "002.jpg"), 640, 480)
	writeJPEG(t, filepath.Join(input, "003.jpg"), 480, 640)

	output := filepath.Join(t.TempDir(), "chapter.epub")

	var buf bytes.Buffer
	root := cli.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"convert", "--input", input, "--format", "epub", "--output", output})

	err := root.Execute()
	require.NoError(t, err)

	fi, err := os.Stat(output)
	require.NoError(t, err)
	require.Greater(t, fi.Size(), int64(0))
}

func TestConvertCommand_RootDirectorySplitsByMergeLimits(t *testing.T) {
	rootInput := t.TempDir()
	for i := 1; i <= 3; i++ {
		chapterDir := filepath.Join(rootInput, "chap-"+leftPad3(i))
		require.NoError(t, os.MkdirAll(chapterDir, 0o755))
		writeJPEG(t, filepath.Join(chapterDir, "001.jpg"), 320, 480)
		writeJPEG(t, filepath.Join(chapterDir, "002.jpg"), 320, 480)
	}

	outputDir := filepath.Join(t.TempDir(), "volumes")

	var buf bytes.Buffer
	root := cli.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"convert",
		"--input", rootInput,
		"--format", "pdf",
		"--output", outputDir,
		"--max-pages", "3",
		"--max-size-mb", "-1",
		"--max-chapters", "-1",
	})

	err := root.Execute()
	require.NoError(t, err)

	entries, err := os.ReadDir(outputDir)
	require.NoError(t, err)
	require.Len(t, entries, 3)
}

func writeJPEG(t *testing.T, path string, w, h int) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 120, G: 24, B: 220, A: 255})
		}
	}
	require.NoError(t, jpeg.Encode(f, img, &jpeg.Options{Quality: 85}))
}

func leftPad3(n int) string {
	if n < 10 {
		return "00" + string(rune('0'+n))
	}
	if n < 100 {
		return "0" + convertItoa(n)
	}
	return convertItoa(n)
}

func convertItoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+(n%10))) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
