package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertInputKind(t *testing.T) {
	chapterDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(chapterDir, "001.jpg"), []byte("x"), 0o600))

	kind, err := convertInputKind(chapterDir)
	require.NoError(t, err)
	require.Equal(t, "chapter", kind)

	rootDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, "chap-001"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "chap-001", "001.jpg"), []byte("x"), 0o600))

	kind, err = convertInputKind(rootDir)
	require.NoError(t, err)
	require.Equal(t, "root", kind)
}

func TestPlanVolumes_MaxPages(t *testing.T) {
	chapters := []chapterInfo{
		{Name: "chap-001", Pages: 2, SizeBytes: 100},
		{Name: "chap-002", Pages: 2, SizeBytes: 100},
		{Name: "chap-003", Pages: 2, SizeBytes: 100},
	}
	volumes, warnings := planVolumes(chapters, mergeLimits{MaxPages: 3, MaxFileSizeMB: -1, MaxChapters: -1})
	require.Empty(t, warnings)
	require.Len(t, volumes, 3)
	require.Equal(t, "chap-001", volumes[0].Chapters[0].Name)
	require.Equal(t, "chap-002", volumes[1].Chapters[0].Name)
	require.Equal(t, "chap-003", volumes[2].Chapters[0].Name)
}

func TestPlanVolumes_OversizedSingleChapter(t *testing.T) {
	chapters := []chapterInfo{{Name: "chap-001", Pages: 600, SizeBytes: 1000}, {Name: "chap-002", Pages: 10, SizeBytes: 1000}}
	volumes, warnings := planVolumes(chapters, mergeLimits{MaxPages: 500, MaxFileSizeMB: -1, MaxChapters: -1})
	require.Len(t, volumes, 2)
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "placed alone")
}

func TestResolveVolumeOutputPath(t *testing.T) {
	got := resolveVolumeOutputPath("/tmp/out/chapter.pdf", "/tmp/in/manga", "pdf", 2, 3)
	require.Equal(t, "/tmp/out/chapter-vol-002.pdf", got)

	got = resolveVolumeOutputPath("/tmp/out-dir", "/tmp/in/My-Manga", "epub", 1, 2)
	require.Equal(t, "/tmp/out-dir/my-manga-vol-001.epub", got)
}
