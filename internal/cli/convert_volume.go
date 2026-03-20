package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ducminhgd/manga-chef/internal/converter"
)

const (
	defaultMaxFileSizeMB = 200
	defaultMaxPages      = 500
	defaultMaxChapters   = 30
)

type mergeLimits struct {
	MaxFileSizeMB int
	MaxPages      int
	MaxChapters   int
}

type chapterInfo struct {
	Name      string
	Path      string
	Number    float64
	HasNumber bool
	Pages     int
	SizeBytes int64
	Images    []string
}

type volumePlan struct {
	Index      int
	Chapters   []chapterInfo
	TotalPages int
	TotalBytes int64
}

func normalizeMergeLimits(l mergeLimits) mergeLimits {
	if l.MaxFileSizeMB == 0 {
		l.MaxFileSizeMB = defaultMaxFileSizeMB
	}
	if l.MaxPages == 0 {
		l.MaxPages = defaultMaxPages
	}
	if l.MaxChapters == 0 {
		l.MaxChapters = defaultMaxChapters
	}
	return l
}

func convertInputKind(inputDir string) (string, error) {
	images, err := collectImagesInDir(inputDir)
	if err != nil {
		return "", err
	}
	if len(images) > 0 {
		return "chapter", nil
	}

	chapters, err := collectChapterDirs(inputDir)
	if err != nil {
		return "", err
	}
	if len(chapters) > 0 {
		return "root", nil
	}
	return "", errors.New("input directory has neither images nor chapter sub-directories")
}

func collectChapterDirs(root string) ([]chapterInfo, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading input directory %q: %w", root, err)
	}

	chapters := make([]chapterInfo, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		chPath := filepath.Join(root, entry.Name())
		images, err := collectImagesInDir(chPath)
		if err != nil {
			return nil, err
		}
		if len(images) == 0 {
			continue
		}
		sz, err := totalFileSize(images)
		if err != nil {
			return nil, err
		}
		num, hasNum := extractChapterNumber(entry.Name())
		chapters = append(chapters, chapterInfo{
			Name:      entry.Name(),
			Path:      chPath,
			Number:    num,
			HasNumber: hasNum,
			Pages:     len(images),
			SizeBytes: sz,
			Images:    images,
		})
	}

	sort.SliceStable(chapters, func(i, j int) bool {
		a, b := chapters[i], chapters[j]
		if a.HasNumber && b.HasNumber && a.Number != b.Number {
			return a.Number < b.Number
		}
		if a.HasNumber != b.HasNumber {
			return a.HasNumber
		}
		return a.Name < b.Name
	})
	return chapters, nil
}

func extractChapterNumber(name string) (float64, bool) {
	clean := strings.ToLower(strings.TrimSpace(name))
	clean = strings.ReplaceAll(clean, "_", "-")
	prefixes := []string{"chapter-", "chap-", "ch-"}
	for _, p := range prefixes {
		if strings.HasPrefix(clean, p) {
			n, err := strconv.ParseFloat(strings.TrimPrefix(clean, p), 64)
			if err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

func collectImagesInDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %q: %w", dir, err)
	}
	images := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".webp":
			images = append(images, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(images)
	return images, nil
}

func totalFileSize(paths []string) (int64, error) {
	var total int64
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			return 0, fmt.Errorf("stat %q: %w", p, err)
		}
		total += st.Size()
	}
	return total, nil
}

func planVolumes(chapters []chapterInfo, raw mergeLimits) ([]volumePlan, []string) {
	limits := normalizeMergeLimits(raw)
	volumes := make([]volumePlan, 0)
	warnings := make([]string, 0)

	current := volumePlan{Index: 1, Chapters: make([]chapterInfo, 0)}
	flush := func() {
		if len(current.Chapters) == 0 {
			return
		}
		volumes = append(volumes, current)
		current = volumePlan{Index: len(volumes) + 1, Chapters: make([]chapterInfo, 0)}
	}

	for _, ch := range chapters {
		if len(current.Chapters) > 0 && wouldExceed(current, ch, limits) {
			flush()
		}

		current.Chapters = append(current.Chapters, ch)
		current.TotalPages += ch.Pages
		current.TotalBytes += ch.SizeBytes

		if len(current.Chapters) == 1 && chapterExceedsLimits(ch, limits) {
			warnings = append(warnings, fmt.Sprintf("chapter %q exceeds active limit(s); placed alone in volume %d", ch.Name, current.Index))
			flush()
		}
	}
	flush()
	return volumes, warnings
}

func wouldExceed(v volumePlan, ch chapterInfo, l mergeLimits) bool {
	nextPages := v.TotalPages + ch.Pages
	nextChapters := len(v.Chapters) + 1
	nextBytes := v.TotalBytes + ch.SizeBytes

	if l.MaxPages >= 0 && nextPages > l.MaxPages {
		return true
	}
	if l.MaxChapters >= 0 && nextChapters > l.MaxChapters {
		return true
	}
	if l.MaxFileSizeMB >= 0 && nextBytes > int64(l.MaxFileSizeMB)*1024*1024 {
		return true
	}
	return false
}

func chapterExceedsLimits(ch chapterInfo, l mergeLimits) bool {
	if l.MaxPages >= 0 && ch.Pages > l.MaxPages {
		return true
	}
	if l.MaxChapters >= 0 && 1 > l.MaxChapters {
		return true
	}
	if l.MaxFileSizeMB >= 0 && ch.SizeBytes > int64(l.MaxFileSizeMB)*1024*1024 {
		return true
	}
	return false
}

func convertMangaRoot(
	ctx context.Context,
	conv converter.ConverterInterface,
	inputDir string,
	outputPath string,
	format string,
	title string,
	limits mergeLimits,
	out io.Writer,
) error {
	chapters, err := collectChapterDirs(inputDir)
	if err != nil {
		return err
	}
	if len(chapters) == 0 {
		return errors.New("no chapter directories with images were found")
	}

	volumes, warnings := planVolumes(chapters, limits)
	for _, w := range warnings {
		fmt.Fprintf(out, "warning: %s\n", w)
	}

	for _, volume := range volumes {
		tmpDir, err := buildVolumeTempDir(volume)
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		target := resolveVolumeOutputPath(outputPath, inputDir, format, volume.Index, len(volumes))
		volTitle := volumeTitle(title, inputDir, volume.Index, len(volumes))
		if err := conv.Convert(ctx, tmpDir, target, converter.Options{Title: volTitle}); err != nil {
			return fmt.Errorf("converting volume %d: %w", volume.Index, err)
		}
		fmt.Fprintf(out, "Converted volume %03d -> %s (%s, %d chapters, %d pages)\n", volume.Index, target, format, len(volume.Chapters), volume.TotalPages)
	}
	return nil
}

func buildVolumeTempDir(volume volumePlan) (string, error) {
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("manga-chef-vol-%03d-*", volume.Index))
	if err != nil {
		return "", fmt.Errorf("creating temporary volume directory: %w", err)
	}
	page := 1
	for _, ch := range volume.Chapters {
		for _, imgPath := range ch.Images {
			ext := strings.ToLower(filepath.Ext(imgPath))
			target := filepath.Join(tmpDir, fmt.Sprintf("%05d%s", page, ext))
			if err := copyFilePath(imgPath, target); err != nil {
				return "", err
			}
			page++
		}
	}
	return tmpDir, nil
}

func copyFilePath(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %q: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("creating %q: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying %q -> %q: %w", src, dst, err)
	}
	return nil
}

func resolveVolumeOutputPath(outputPath, inputDir, format string, index, total int) string {
	ext := "." + strings.ToLower(format)
	cleanOut := filepath.Clean(outputPath)
	if strings.EqualFold(filepath.Ext(cleanOut), ext) {
		if total == 1 {
			return cleanOut
		}
		base := strings.TrimSuffix(cleanOut, filepath.Ext(cleanOut))
		return fmt.Sprintf("%s-vol-%03d%s", base, index, ext)
	}

	manga := sanitizeSlug(filepath.Base(filepath.Clean(inputDir)))
	return filepath.Join(cleanOut, fmt.Sprintf("%s-vol-%03d%s", manga, index, ext))
}

func volumeTitle(userTitle, inputDir string, index, total int) string {
	base := strings.TrimSpace(userTitle)
	if base == "" {
		base = prettifySegment(filepath.Base(filepath.Clean(inputDir)))
		if base == "" {
			base = "Manga"
		}
	}
	if total <= 1 {
		return base
	}
	return fmt.Sprintf("%s - Volume %d", base, index)
}

func prettifySegment(s string) string {
	clean := strings.TrimSpace(s)
	clean = strings.ReplaceAll(clean, "_", " ")
	clean = strings.ReplaceAll(clean, "-", " ")
	return strings.Join(strings.Fields(clean), " ")
}
