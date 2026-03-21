package converter

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	_ "golang.org/x/image/webp"
)

// DetectImageExtension returns the canonical file extension for the image bytes at path.
func DetectImageExtension(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()

	_, format, err := image.DecodeConfig(f)
	if err != nil {
		return "", fmt.Errorf("decoding %q: %w", path, err)
	}

	switch format {
	case "jpeg":
		return ".jpg", nil
	case "png":
		return ".png", nil
	case "webp":
		return ".webp", nil
	default:
		return "", fmt.Errorf("unsupported image format %q in %q", format, path)
	}
}
