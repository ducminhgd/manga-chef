// Package cli wires all cobra sub-commands into the root command.
// It is the only place that calls os.Exit — all RunE functions return errors
// which the root command converts to non-zero exit codes automatically.
package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	_ "github.com/ducminhgd/manga-chef/internal/scraper/truyenqq"
	"github.com/ducminhgd/manga-chef/internal/version"
)

// rootFlags holds the values of the persistent root-level flags so they can
// be read by any sub-command without global variables.
type rootFlags struct {
	sourcesPath string
	outputDir   string
	logLevel    string
}

// NewRootCmd constructs and returns the fully wired root cobra command.
// Callers should call Execute() on the returned command.
func NewRootCmd() *cobra.Command {
	f := &rootFlags{}

	root := &cobra.Command{
		Use:   "manga-chef",
		Short: "A source-agnostic manga downloader and converter",
		Long: `Manga Chef downloads manga chapters from configurable sources and converts
them to PDF, EPUB, or MOBI for offline reading.

Sources are defined in YAML files (see: manga-chef sources --help).
Download chapters with: manga-chef download --source <code> --url <manga-url>`,
		Version:       buildVersion(),
		SilenceUsage:  true, // Don't print usage on RunE errors (just the error)
		SilenceErrors: false,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return configureLogger(f.logLevel)
		},
	}

	// ── Persistent flags (available to every sub-command) ───────────────────
	root.PersistentFlags().StringVar(
		&f.sourcesPath,
		"sources",
		defaultSourcesPath(),
		"Path to a sources YAML file or directory",
	)
	root.PersistentFlags().StringVar(
		&f.outputDir,
		"output",
		"./library",
		"Directory where downloaded chapters and converted files are written",
	)
	root.PersistentFlags().StringVar(
		&f.logLevel,
		"log-level",
		"info",
		`Log verbosity: debug, info, warn, error`,
	)

	// ── Sub-commands ────────────────────────────────────────────────────────
	root.AddCommand(NewSourcesCmd(func() string { return f.sourcesPath }))

	return root
}

// Execute is the single entry point called by main(). It runs the root command
// and exits with code 1 on error.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// buildVersion returns the version string shown by --version.
func buildVersion() string {
	if version.Commit == "unknown" {
		return version.Version
	}
	return fmt.Sprintf("%s (commit %s, built %s)", version.Version, version.Commit, version.BuildDate)
}

// defaultSourcesPath returns the default location for the sources file.
// It prefers XDG_CONFIG_HOME when set, otherwise falls back to ~/.config.
func defaultSourcesPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "./sources"
		}
		configHome = home + "/.config"
	}
	return configHome + "/manga-chef/sources"
}

// configureLogger initialises the global slog logger with the requested level.
func configureLogger(level string) error {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "info":
		l = slog.LevelInfo
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		return fmt.Errorf("invalid log level %q — use: debug, info, warn, error", level)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l})))
	return nil
}
