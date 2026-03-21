package cli_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ducminhgd/manga-chef/internal/cli"
)

func TestMergeCommand_CreatesVolumeDirectory(t *testing.T) {
	input := t.TempDir()
	createChapter(t, input, 1, 2)
	createChapter(t, input, 2, 2)

	output := filepath.Join(t.TempDir(), "merged")

	var buf bytes.Buffer
	root := cli.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--output", output, "merge", "--input", input, "--max-size-mb", "-1", "--max-pages", "10", "--max-chapters", "-1"})

	err := root.Execute()
	require.NoError(t, err)

	volDir := filepath.Join(output, "VOL_001_C1-C2")
	entries, err := os.ReadDir(volDir)
	require.NoError(t, err)
	require.Len(t, entries, 4)
}

func TestMergeCommand_SplitsVolumesByLimit(t *testing.T) {
	input := t.TempDir()
	createChapter(t, input, 1, 2)
	createChapter(t, input, 2, 2)
	createChapter(t, input, 3, 2)

	output := filepath.Join(t.TempDir(), "merged")

	var buf bytes.Buffer
	root := cli.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--output", output, "merge", "--input", input, "--max-size-mb", "-1", "--max-pages", "3", "--max-chapters", "-1"})

	err := root.Execute()
	require.NoError(t, err)

	for _, d := range []string{"VOL_001_C1-C1", "VOL_002_C2-C2", "VOL_003_C3-C3"} {
		_, err := os.Stat(filepath.Join(output, d))
		require.NoError(t, err)
	}
}

func TestMergeCommand_DeleteMergedChapters(t *testing.T) {
	input := t.TempDir()
	createChapter(t, input, 1, 2)
	createChapter(t, input, 2, 2)

	var buf bytes.Buffer
	root := cli.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"merge", "--input", input, "--max-size-mb", "-1", "--max-pages", "10", "--max-chapters", "-1", "--delete-merged-chapters"})

	err := root.Execute()
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(input, "chap-001"))
	require.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(input, "chap-002"))
	require.True(t, os.IsNotExist(err))
}

func TestMergeCommand_ConvertOptionPDF(t *testing.T) {
	input := t.TempDir()
	createChapter(t, input, 1, 2)
	createChapter(t, input, 2, 2)

	output := filepath.Join(t.TempDir(), "merged")

	var buf bytes.Buffer
	root := cli.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--output", output, "merge", "--input", input, "--max-size-mb", "-1", "--max-pages", "10", "--max-chapters", "-1", "--convert", "pdf"})

	err := root.Execute()
	require.NoError(t, err)

	pdfPath := filepath.Join(output, "VOL_001_C1-C2.pdf")
	st, err := os.Stat(pdfPath)
	require.NoError(t, err)
	require.Greater(t, st.Size(), int64(0))
}

func TestMergeCommand_ConvertOptionPDF_WithMislabeledImage(t *testing.T) {
	input := t.TempDir()
	chapterDir := filepath.Join(input, "chap-001")
	require.NoError(t, os.MkdirAll(chapterDir, 0o755))
	writePNGWithJPGName(t, filepath.Join(chapterDir, "001.jpg"), 320, 480)
	writeJPEG(t, filepath.Join(chapterDir, "002.jpg"), 320, 480)

	output := filepath.Join(t.TempDir(), "merged")

	var buf bytes.Buffer
	root := cli.NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--output", output, "merge", "--input", input, "--max-size-mb", "-1", "--max-pages", "10", "--max-chapters", "-1", "--convert", "pdf"})

	err := root.Execute()
	require.NoError(t, err)

	pdfPath := filepath.Join(output, "VOL_001_C1-C1.pdf")
	st, err := os.Stat(pdfPath)
	require.NoError(t, err)
	require.Greater(t, st.Size(), int64(0))

	_, err = os.Stat(filepath.Join(output, "VOL_001_C1-C1", "00001.png"))
	require.NoError(t, err)
}

func createChapter(t *testing.T, root string, chapterNum, pages int) {
	t.Helper()
	chapterDir := filepath.Join(root, "chap-"+leftPad3(chapterNum))
	require.NoError(t, os.MkdirAll(chapterDir, 0o755))
	for i := 1; i <= pages; i++ {
		writeJPEG(t, filepath.Join(chapterDir, leftPad3(i)+".jpg"), 320, 480)
	}
}

func writePNGWithJPGName(t *testing.T, path string, w, h int) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 24, G: 160, B: 120, A: 255})
		}
	}
	require.NoError(t, png.Encode(f, img))
}
