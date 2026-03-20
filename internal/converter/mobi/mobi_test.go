package mobi

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ducminhgd/manga-chef/internal/converter"
)

type fakeEPUBConverter struct {
	calledInput  string
	calledOutput string
	calledOpts   converter.Options
	err          error
}

func (f *fakeEPUBConverter) Convert(_ context.Context, inputDir, outputPath string, opts converter.Options) error {
	f.calledInput = inputDir
	f.calledOutput = outputPath
	f.calledOpts = opts
	if f.err != nil {
		return f.err
	}
	return os.WriteFile(outputPath, []byte("epub"), 0o600)
}

func TestConvert_RunsEbookConvertWithExpectedArgs(t *testing.T) {
	fakeEPUB := &fakeEPUBConverter{}

	var calledName string
	var calledArgs []string
	conv := &Converter{
		epub: fakeEPUB,
		lookPath: func(file string) (string, error) {
			require.Equal(t, calibreBinary, file)
			return "/usr/bin/ebook-convert", nil
		},
		run: func(_ context.Context, name string, args ...string) error {
			calledName = name
			calledArgs = append([]string(nil), args...)
			return os.WriteFile(args[1], []byte("mobi"), 0o600)
		},
	}

	input := t.TempDir()
	output := filepath.Join(t.TempDir(), "chapter.mobi")

	err := conv.Convert(context.Background(), input, output, converter.Options{Title: "Demo"})
	require.NoError(t, err)

	require.Equal(t, input, fakeEPUB.calledInput)
	require.Equal(t, converter.Options{Title: "Demo"}, fakeEPUB.calledOpts)
	require.Equal(t, calibreBinary, calledName)
	require.Len(t, calledArgs, 2)
	require.Equal(t, output, calledArgs[1])
	require.Equal(t, ".epub", filepath.Ext(calledArgs[0]))
	require.Equal(t, calledArgs[0], fakeEPUB.calledOutput)
	require.FileExists(t, output)
}

func TestConvert_ReturnsClearErrorWhenCalibreMissing(t *testing.T) {
	conv := &Converter{
		epub: &fakeEPUBConverter{},
		lookPath: func(_ string) (string, error) {
			return "", exec.ErrNotFound
		},
		run: func(_ context.Context, _ string, _ ...string) error {
			return nil
		},
	}

	err := conv.Convert(context.Background(), t.TempDir(), filepath.Join(t.TempDir(), "out.mobi"), converter.Options{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "ebook-convert not found")
	require.Contains(t, err.Error(), calibreURL)
}

func TestConvert_PropagatesEbookConvertError(t *testing.T) {
	fakeEPUB := &fakeEPUBConverter{}
	conv := &Converter{
		epub: fakeEPUB,
		lookPath: func(_ string) (string, error) {
			return "/usr/bin/ebook-convert", nil
		},
		run: func(_ context.Context, _ string, _ ...string) error {
			return errors.New("conversion failed")
		},
	}

	err := conv.Convert(context.Background(), t.TempDir(), filepath.Join(t.TempDir(), "out.mobi"), converter.Options{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "running ebook-convert")
}
