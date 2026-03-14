// Package config loads and validates manga source configurations from YAML files.
//
// # File format
//
// A sources file is a YAML document with a top-level "sources" key:
//
//	sources:
//	  - name: "TruyenQQ"
//	    code: "truyenqq"
//	    base_url: "https://truyenqqto.com"
//	    scraper: "truyenqq"
//	    rate_limit_ms: 500
//
// # Loading
//
// Use Load to load from a file or directory:
//
//	cfgs, err := config.Load("./sources")
//
// All validation errors are collected before returning, so callers see the
// complete picture in a single error rather than fixing one issue at a time.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	yaml "gopkg.in/yaml.v3"

	"github.com/ducminhgd/manga-chef/pkg/sources"
)

// sourcesFile is the top-level structure of a sources YAML document.
type sourcesFile struct {
	Sources []sources.SourceConfig `yaml:"sources"`
}

// Load loads SourceConfig values from path.
//
//   - If path points to a regular file, that file is loaded.
//   - If path points to a directory, all *.yml and *.yaml files inside it are
//     loaded in lexicographic order (non-recursive).
//
// Duplicate source codes across multiple files in a directory are reported as
// validation errors. All issues are collected before returning.
func Load(path string) ([]sources.SourceConfig, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("accessing sources path %q: %w", path, err)
	}
	if info.IsDir() {
		return LoadDir(path)
	}
	return LoadFile(path)
}

// LoadFile loads and validates SourceConfig values from a single YAML file.
// It returns ValidationErrors when one or more sources fail validation rules.
func LoadFile(path string) ([]sources.SourceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading sources file %q: %w", path, err)
	}
	return parseFile(data, path, defaultValidator())
}

// LoadDir loads and validates SourceConfig values from all *.yml / *.yaml files
// in dir (non-recursive, lexicographic order).
//
// Duplicate source codes across files are treated as errors. All issues from
// all files are collected and returned together.
func LoadDir(dir string) ([]sources.SourceConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading sources directory %q: %w", dir, err)
	}

	yamlFiles := collectYAMLFiles(entries, dir)
	if len(yamlFiles) == 0 {
		return nil, nil
	}

	results, allErrs, err := parseSourceFiles(yamlFiles)
	if err != nil {
		return nil, err
	}

	all, allErrs := flattenAndCheckDuplicates(results, allErrs)
	if allErrs.HasErrors() {
		return all, allErrs
	}
	if len(allErrs) > 0 {
		return all, allErrs
	}
	return all, nil
}

func collectYAMLFiles(entries []os.DirEntry, dir string) []string {
	yamlFiles := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext == ".yml" || ext == ".yaml" {
			yamlFiles = append(yamlFiles, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(yamlFiles)
	return yamlFiles
}

type fileResult struct {
	file    string
	configs []sources.SourceConfig
}

func parseSourceFiles(files []string) ([]fileResult, ValidationErrors, error) {
	var (
		results []fileResult
		allErrs ValidationErrors
	)

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("reading %q: %w", f, err)
		}

		cfgs, err := parseFile(data, f, defaultValidator())
		if err != nil {
			var ve ValidationErrors
			if errors.As(err, &ve) {
				allErrs = append(allErrs, ve...)
				results = append(results, fileResult{file: f, configs: cfgs})
				continue
			}
			return nil, nil, err
		}
		results = append(results, fileResult{file: f, configs: cfgs})
	}
	return results, allErrs, nil
}

func flattenAndCheckDuplicates(results []fileResult, allErrs ValidationErrors) ([]sources.SourceConfig, ValidationErrors) {
	seenCode := make(map[string]string)
	var all []sources.SourceConfig

	for _, r := range results {
		for _, cfg := range r.configs {
			if first, dup := seenCode[cfg.Code]; dup {
				allErrs = append(allErrs, ValidationError{
					Severity: SeverityError,
					File:     r.file,
					Field:    "sources[].code",
					Message:  fmt.Sprintf("duplicate source code %q — already defined in %s", cfg.Code, first),
				})
			} else {
				seenCode[cfg.Code] = r.file
			}
			all = append(all, cfg)
		}
	}

	return all, allErrs
}

// parseFile decodes data as a sources YAML document and runs validation.
// It returns the parsed configs (even when there are errors, so callers such
// as LoadDir can continue with cross-file duplicate checks) alongside any
// ValidationErrors.
func parseFile(data []byte, filename string, v *Validator) ([]sources.SourceConfig, error) {
	// Decode into yaml.Node first to preserve line/column information that
	// we later attach to validation errors.
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing %q: %w", filename, err)
	}

	// An empty or whitespace-only file is valid; return nothing.
	if root.Kind == 0 {
		return nil, nil
	}

	// Decode into the typed struct via the already-parsed node tree.
	var sf sourcesFile
	if err := root.Decode(&sf); err != nil {
		return nil, fmt.Errorf("decoding %q: %w", filename, err)
	}

	// Extract per-source field line numbers for error reporting.
	nodeInfos := extractNodeInfos(&root)

	errs := v.Validate(sf.Sources, nodeInfos, filename)
	if len(errs) > 0 {
		return sf.Sources, errs
	}
	return sf.Sources, nil
}

// defaultValidator returns a Validator with no pre-registered scraper names.
// Built-in name validation is skipped; only file-path scrapers are checked.
func defaultValidator() *Validator {
	return &Validator{}
}
