// Package cli contains all cobra sub-command definitions for Manga Chef.
// No business logic lives here — this package only wires flags to internal
// packages and formats output for the terminal.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ducminhgd/manga-chef/internal/config"
	"github.com/ducminhgd/manga-chef/pkg/sources"
)

// NewSourcesCmd returns the "sources" parent command with its sub-commands
// attached. The sourcesPath flag is read from the root-level persistent flag
// set and passed in here so tests can inject a custom path without touching
// global state.
func NewSourcesCmd(getSourcesPath func() string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage manga source configurations",
		Long: `Manage the YAML source configurations used by Manga Chef.

Sources define where manga is downloaded from. Each source has a short code
used with --source on other commands (e.g. manga-chef download --source truyenqq).

Source files follow this format:

  sources:
    - name: "TruyenQQ"
      code: "truyenqq"
      base_url: "https://truyenqqto.com"
      scraper: "truyenqq"
      rate_limit_ms: 500`,
	}

	cmd.AddCommand(newSourcesListCmd(getSourcesPath))
	cmd.AddCommand(newSourcesAddCmd(getSourcesPath))
	return cmd
}

// ── sources list (T-17). ─────────────────────────────────────────────────────────.

func newSourcesListCmd(getSourcesPath func() string) *cobra.Command {
	var showDisabled bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured sources",
		Long: `Print a table of all manga sources found in the sources file or directory.

Disabled sources (enabled: false) are hidden by default. Use --all to show them.`,
		Example: `  manga-chef sources list
  manga-chef sources list --all
  manga-chef sources list --sources ./my-sources.yml`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSourcesList(cmd.OutOrStdout(), getSourcesPath(), showDisabled)
		},
	}

	cmd.Flags().BoolVar(&showDisabled, "all", false, "Include disabled sources in the output")
	return cmd
}

func runSourcesList(w io.Writer, sourcesPath string, showDisabled bool) error {
	cfgs, err := config.Load(sourcesPath)

	// Surface warnings but do not abort — a source with warnings is still usable.
	var ve config.ValidationErrors
	if err != nil && errors.As(err, &ve) {
		if ve.HasErrors() {
			return fmt.Errorf("sources configuration is invalid:\n%w", err)
		}
		// Warnings only: print them above the table so they're visible.
		for _, w2 := range ve.Warnings() {
			fmt.Fprintf(w, "warning: %s\n", w2.String())
		}
	} else if err != nil {
		return fmt.Errorf("loading sources from %q: %w", sourcesPath, err)
	}

	// Filter out disabled sources unless --all was passed.
	visible := make([]sources.SourceConfig, 0, len(cfgs))
	for _, c := range cfgs {
		if c.IsEnabled() || showDisabled {
			visible = append(visible, c)
		}
	}

	if len(visible) == 0 {
		fmt.Fprintln(w, "No sources found. Add one with: manga-chef sources add <file.yml>")
		return nil
	}

	// Tabwriter aligns columns even when values have different widths.
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "CODE\tNAME\tBASE URL\tSCRAPER\tSTATUS")
	fmt.Fprintln(tw, strings.Repeat("-", 6)+"\t"+
		strings.Repeat("-", 4)+"\t"+
		strings.Repeat("-", 8)+"\t"+
		strings.Repeat("-", 7)+"\t"+
		strings.Repeat("-", 6))

	for _, c := range visible {
		status := "enabled"
		if !c.IsEnabled() {
			status = "disabled"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			c.Code, c.Name, c.BaseURL, c.Scraper, status)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("writing source table: %w", err)
	}
	return nil
}

// ── sources add (T-18). ────────────────────────────────────────────────────────.

func newSourcesAddCmd(getSourcesPath func() string) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "add <file.yml>",
		Short: "Add sources from a YAML file",
		Long: `Validate and merge sources from a YAML file into the active sources directory.

The file is validated first. If validation fails, nothing is written.
If a source code defined in the new file already exists in the active
configuration, an error is returned and the merge is aborted.

Use --dry-run to validate and preview what would be added without writing.`,
		Example: `  manga-chef sources add ./sources/truyenqq.yml
  manga-chef sources add ./sources/truyenqq.yml --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSourcesAdd(cmd.OutOrStdout(), args[0], getSourcesPath(), dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate and preview without writing")
	return cmd
}

func runSourcesAdd(w io.Writer, newFile, sourcesPath string, dryRun bool) error {
	newCfgs, err := validateSourceFile(w, newFile)
	if err != nil {
		return err
	}
	if len(newCfgs) == 0 {
		fmt.Fprintf(w, "No sources found in %q — nothing to add.\n", newFile)
		return nil
	}

	existingCfgs, err := loadExisting(sourcesPath)
	if err != nil {
		return err
	}
	if err := checkDuplicateCodes(sourcesPath, existingCfgs, newCfgs); err != nil {
		return err
	}

	fmt.Fprintf(w, "Sources to be added from %q:\n", newFile)
	for _, c := range newCfgs {
		fmt.Fprintf(w, "  + %s (%s) — %s\n", c.Name, c.Code, c.BaseURL)
	}
	if dryRun {
		fmt.Fprintln(w, "\nDry run — no files written.")
		return nil
	}

	dest, err := destinationPath(sourcesPath, newFile)
	if err != nil {
		return err
	}
	if err := copyFile(newFile, dest); err != nil {
		return fmt.Errorf("writing to %q: %w", dest, err)
	}
	fmt.Fprintf(w, "\n✓ Added %d source(s) → %s\n", len(newCfgs), dest)
	return nil
}

func validateSourceFile(w io.Writer, newFile string) ([]sources.SourceConfig, error) {
	newCfgs, err := config.LoadFile(newFile)
	if err != nil {
		var ve config.ValidationErrors
		if errors.As(err, &ve) {
			if ve.HasErrors() {
				return nil, fmt.Errorf("the source file %q has validation errors:\n%w", newFile, err)
			}
			for _, w2 := range ve.Warnings() {
				fmt.Fprintf(w, "warning: %s\n", w2.String())
			}
			return newCfgs, nil
		}
		return nil, fmt.Errorf("reading %q: %w", newFile, err)
	}
	return newCfgs, nil
}

func checkDuplicateCodes(sourcesPath string, existingCfgs, newCfgs []sources.SourceConfig) error {
	existingCodes := make(map[string]bool, len(existingCfgs))
	for _, c := range existingCfgs {
		existingCodes[c.Code] = true
	}

	var duplicates []string
	for _, c := range newCfgs {
		if existingCodes[c.Code] {
			duplicates = append(duplicates, c.Code)
		}
	}
	if len(duplicates) == 0 {
		return nil
	}
	return fmt.Errorf(
		"cannot add: the following source code(s) already exist in %q: %s\n"+
			"Remove or rename them before adding, or edit the existing config directly.",
		sourcesPath, strings.Join(duplicates, ", "),
	)
}

// loadExisting returns the current sources from sourcesPath, ignoring
// validation warnings. It returns nil (not an error) when the path doesn't
// exist yet (first-time setup).
func loadExisting(sourcesPath string) ([]sources.SourceConfig, error) {
	if _, err := os.Stat(sourcesPath); os.IsNotExist(err) {
		return nil, nil
	}
	cfgs, err := config.Load(sourcesPath)
	if err != nil {
		var ve config.ValidationErrors
		if errors.As(err, &ve) && !ve.HasErrors() {
			return cfgs, nil // warnings only
		}
		if errors.As(err, &ve) {
			return cfgs, nil // return what we have for duplicate detection
		}
	}
	return cfgs, nil
}

// destinationPath computes where the incoming file should land.
//
//   - When sourcesPath is a directory: <sourcesPath>/<basename of newFile>
//   - When sourcesPath is a file: error — you can't merge into a single file
//     without potentially overwriting other sources
func destinationPath(sourcesPath, newFile string) (string, error) {
	info, err := os.Stat(sourcesPath)
	if err != nil {
		if os.IsNotExist(err) {
			// sourcesPath doesn't exist yet; treat it as a directory to create.
			if mkErr := os.MkdirAll(sourcesPath, 0o750); mkErr != nil {
				return "", fmt.Errorf("creating sources directory %q: %w", sourcesPath, mkErr)
			}
			return filepath.Join(sourcesPath, filepath.Base(newFile)), nil
		}
		return "", fmt.Errorf("stat %q: %w", sourcesPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf(
			"%q is a file, not a directory — cannot add sources into a single file\n"+
				"Tip: convert it to a directory and re-run.",
			sourcesPath,
		)
	}
	return filepath.Join(sourcesPath, filepath.Base(newFile)), nil
}

// copyFile copies src to dst, creating or truncating dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %q: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return fmt.Errorf("opening %q: %w", dst, err)
	}
	defer out.Close()

	buf := make([]byte, 32*1024)
	for {
		n, readErr := in.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("writing %q: %w", dst, writeErr)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("reading %q: %w", src, readErr)
		}
	}
	if err := out.Sync(); err != nil {
		return fmt.Errorf("syncing %q: %w", dst, err)
	}
	return nil
}
