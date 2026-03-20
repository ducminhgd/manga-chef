package converter

import "context"

// Options carries cross-format conversion settings.
type Options struct {
	Title string
}

// ConverterInterface converts an image directory into an output artifact.
type ConverterInterface interface {
	Convert(ctx context.Context, inputDir, outputPath string, opts Options) error
}
