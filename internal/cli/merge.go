package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ducminhgd/manga-chef/internal/converter"
)

func newMergeCmd(getOutputPath func() string) *cobra.Command {
	var inputDir string
	var maxSizeMB int
	var maxPages int
	var maxChapters int
	var deleteMergedChapters bool
	var convertFormat string
	var title string

	cmd := &cobra.Command{
		Use:   "merge",
		Short: "Merge chapter directories into volume directories",
		Long: `Merge chapter directories into one or more volume directories.

Volume directory names follow: VOL_<VolumeSequence>_C<FromChapter>-C<ToChapter>.
You can optionally convert each merged volume to pdf/epub/mobi and optionally
remove merged chapter directories after successful merge.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(inputDir) == "" {
				return errors.New("--input is required")
			}

			chapters, err := collectChapterDirs(inputDir)
			if err != nil {
				return err
			}
			if len(chapters) == 0 {
				return errors.New("no chapter directories with images were found")
			}

			limits := mergeLimits{MaxFileSizeMB: maxSizeMB, MaxPages: maxPages, MaxChapters: maxChapters}
			volumes, warnings := planVolumes(chapters, limits)
			for _, w := range warnings {
				fmt.Fprintf(cmd.OutOrStdout(), "warning: %s\n", w)
			}

			outputRoot := strings.TrimSpace(getOutputPath())
			if outputRoot == "" {
				outputRoot = inputDir
			}
			if err := os.MkdirAll(outputRoot, 0o755); err != nil {
				return fmt.Errorf("creating output root %q: %w", outputRoot, err)
			}

			var conv converter.ConverterInterface
			formats := make([]string, 0)
			if strings.TrimSpace(convertFormat) != "" {
				formats, err = parseFormats(convertFormat)
				if err != nil {
					return err
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			for _, vol := range volumes {
				name := volumeDirName(vol)
				volDir := filepath.Join(outputRoot, name)
				if err := os.MkdirAll(volDir, 0o755); err != nil {
					return fmt.Errorf("creating volume directory %q: %w", volDir, err)
				}
				if err := materializeVolumeIntoDir(vol, volDir); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Merged volume %03d -> %s (%d chapters, %d pages)\n", vol.Index, volDir, len(vol.Chapters), vol.TotalPages)

				if len(formats) > 0 {
					for _, format := range formats {
						conv, err = newConverterByFormat(format)
						if err != nil {
							return err
						}
						volTitle := title
						if strings.TrimSpace(volTitle) == "" {
							volTitle = volumeTitle("", inputDir, vol.Index, len(volumes))
						}
						fileName := name
						if strings.TrimSpace(title) != "" {
							fileName = sanitizeSlug(title) + "-" + name
						}
						outFile := filepath.Join(outputRoot, fileName+"."+format)
						if err := conv.Convert(ctx, volDir, outFile, converter.Options{Title: volTitle}); err != nil {
							return fmt.Errorf("converting merged volume %q: %w", name, err)
						}
						fmt.Fprintf(cmd.OutOrStdout(), "Converted %s -> %s (%s)\n", volDir, outFile, format)
					}
				}

				if deleteMergedChapters {
					for _, ch := range vol.Chapters {
						if err := os.RemoveAll(ch.Path); err != nil {
							return fmt.Errorf("deleting merged chapter directory %q: %w", ch.Path, err)
						}
					}
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputDir, "input", "", "Root directory containing chapter sub-directories")
	cmd.Flags().IntVar(&maxSizeMB, "max-size-mb", defaultMaxFileSizeMB, "Maximum output size per volume in MB (-1 to disable)")
	cmd.Flags().IntVar(&maxPages, "max-pages", defaultMaxPages, "Maximum pages per volume (-1 to disable)")
	cmd.Flags().IntVar(&maxChapters, "max-chapters", defaultMaxChapters, "Maximum chapters per volume (-1 to disable)")
	cmd.Flags().BoolVar(&deleteMergedChapters, "delete-merged-chapters", false, "Delete chapter directories that were merged")
	cmd.Flags().StringVar(&convertFormat, "convert", "", "Optional format conversion after merge: pdf, epub, mobi")
	cmd.Flags().StringVar(&title, "title", "", "Optional title metadata for converted outputs")
	return cmd
}

func volumeDirName(v volumePlan) string {
	if len(v.Chapters) == 0 {
		return fmt.Sprintf("VOL_%03d_C0-C0", v.Index)
	}
	from := chapterLabelForName(v.Chapters[0])
	to := chapterLabelForName(v.Chapters[len(v.Chapters)-1])
	return fmt.Sprintf("VOL_%03d_C%s-C%s", v.Index, from, to)
}

func chapterLabelForName(ch chapterInfo) string {
	if ch.HasNumber {
		if ch.Number == float64(int64(ch.Number)) {
			return strconv.FormatInt(int64(ch.Number), 10)
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%g", ch.Number), "0"), ".")
	}
	name := strings.TrimSpace(ch.Name)
	if name == "" {
		return "0"
	}
	return sanitizeSlug(name)
}

func materializeVolumeIntoDir(v volumePlan, outDir string) error {
	page := 1
	for _, ch := range v.Chapters {
		for _, imgPath := range ch.Images {
			ext, err := converter.DetectImageExtension(imgPath)
			if err != nil {
				return fmt.Errorf("detecting image format for %q: %w", imgPath, err)
			}
			target := filepath.Join(outDir, fmt.Sprintf("%05d%s", page, ext))
			if err := copyFilePath(imgPath, target); err != nil {
				return err
			}
			page++
		}
	}
	return nil
}
