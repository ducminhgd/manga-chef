// Package version exposes build-time version metadata.
// The variables are set by the linker via -X flags in the Makefile LDFLAGS.
package version

import "fmt"

// These variables are overwritten at link time by:
//
//	-X github.com/ducminhgd/manga-chef/internal/version.Version=<tag>
//	-X github.com/ducminhgd/manga-chef/internal/version.Commit=<sha>
//	-X github.com/ducminhgd/manga-chef/internal/version.BuildDate=<date>
var (
	// Version is the semantic version tag (e.g. "v1.2.3"). Defaults to "dev".
	Version = "dev"

	// Commit is the short Git commit SHA at build time. Defaults to "unknown".
	Commit = "unknown"

	// BuildDate is the UTC timestamp of the build in RFC 3339 format. Defaults to "unknown".
	BuildDate = "unknown"
)

// String returns a human-readable version string suitable for --version output.
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, BuildDate)
}
