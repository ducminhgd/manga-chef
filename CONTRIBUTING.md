# Contributing to Manga Chef

Thank you for your interest in contributing to Manga Chef! This document covers everything you need to get your development environment set up, understand the project conventions, and get your changes merged.

---

## Table of Contents

1. [Code of Conduct](#1-code-of-conduct)
2. [Getting Started](#2-getting-started)
3. [Project Structure](#3-project-structure)
4. [Development Workflow](#4-development-workflow)
5. [Coding Conventions](#5-coding-conventions)
6. [Testing](#6-testing)
7. [Adding a New Source Scraper](#7-adding-a-new-source-scraper)
8. [Commit Messages](#8-commit-messages)
9. [Pull Request Guidelines](#9-pull-request-guidelines)
10. [Reporting Issues](#10-reporting-issues)

---

## 1. Code of Conduct

Be respectful and constructive. We welcome contributors of all experience levels. Harassment, gatekeeping, or dismissive behavior toward any contributor will not be tolerated.

---

## 2. Getting Started

### Prerequisites

| Tool | Minimum Version | Notes |
|---|---|---|
| Go | 1.22 | Use the version in `go.mod` as the source of truth |
| Git | 2.x | |
| Make | any | Optional but recommended |
| Calibre | any | Optional — required only for MOBI conversion (`ebook-convert`) |

### Fork and Clone

```bash
# Fork the repository on GitHub, then:
git clone https://github.com/<your-username>/manga-chef.git
cd manga-chef

# Add the upstream remote
git remote add upstream https://github.com/ducminhgd/manga-chef.git
```

### Install Dependencies

```bash
go mod download
```

### Build

```bash
go build -o manga-chef ./cmd/manga-chef
```

Or via Make:

```bash
make build
```

### Run Locally

```bash
./manga-chef --help
```

---

## 3. Project Structure

```
manga-chef/
├── cmd/
│   └── manga-chef/         # Entry point — cobra root command
│       └── main.go
├── internal/
│   ├── config/             # YAML source config loading and validation
│   ├── scraper/            # Scraper interface + built-in implementations
│   │   ├── scraper.go      # ScraperInterface definition
│   │   ├── registry.go     # Scraper registration by code name
│   │   ├── generic/        # CSS-selector-based generic scraper
│   │   └── mangadex/       # MangaDex API scraper
│   ├── downloader/         # Concurrent image downloading logic
│   ├── converter/          # PDF, EPUB, MOBI conversion
│   └── cli/                # Cobra sub-command definitions
├── pkg/
│   └── sources/            # Public types: Source, Chapter, ImageURL
├── sources/                # Bundled community source YAML files
├── scripts/                # Dev utility scripts (lint, test, release)
├── testdata/               # Fixtures and golden files for tests
├── go.mod
├── go.sum
├── Makefile
└── sources.example.yml     # Annotated example source configuration
```

**Key architectural rule:** `internal/` packages are not importable by external consumers. Public types shared across packages live in `pkg/`. The `cmd/` layer only wires together `internal/cli` — it contains no business logic.

---

## 4. Development Workflow

### Create a Feature Branch

Always branch off `main`:

```bash
git checkout main
git pull upstream main
git checkout -b feat/my-feature-name
```

Branch naming convention:

| Type | Prefix | Example |
|---|---|---|
| New feature | `feat/` | `feat/nettruyen-scraper` |
| Bug fix | `fix/` | `fix/epub-cover-missing` |
| Refactor | `refactor/` | `refactor/downloader-retry-logic` |
| Documentation | `docs/` | `docs/update-contributing` |
| Tests | `test/` | `test/generic-scraper-coverage` |
| CI / tooling | `chore/` | `chore/update-golangci-config` |

### Keep Your Branch Up to Date

```bash
git fetch upstream
git rebase upstream/main
```

Prefer rebase over merge to keep a linear history.

---

## 5. Coding Conventions

### General

- Follow standard [Effective Go](https://go.dev/doc/effective_go) principles.
- Keep functions small and focused. If a function needs a comment to explain *what* it does (not *why*), consider splitting it.
- Prefer explicit error handling over `panic`. Never swallow errors silently.
- Use `context.Context` as the first parameter of any function that performs I/O or could be long-running.

### Error Handling

Wrap errors with context using `fmt.Errorf`:

```go
// Good
if err != nil {
    return fmt.Errorf("fetching chapter list for %s: %w", url, err)
}

// Bad — loses context
if err != nil {
    return err
}
```

Define sentinel errors in the package they originate from:

```go
var ErrChapterNotFound = errors.New("chapter not found")
```

### Interfaces

Define interfaces in the package that *consumes* them, not the package that implements them. This is Go's implicit interface idiom and keeps dependencies pointing inward.

```go
// internal/downloader/downloader.go — consumer defines the interface
type Scraper interface {
    GetChapterList(ctx context.Context, mangaURL string) ([]sources.Chapter, error)
    GetImageURLs(ctx context.Context, chapterURL string) ([]string, error)
}
```

### Naming

| Thing | Convention | Example |
|---|---|---|
| Packages | lowercase, single word | `scraper`, `converter` |
| Exported types | PascalCase | `ChapterList`, `SourceConfig` |
| Unexported vars | camelCase | `retryCount` |
| Acronyms | all-caps if exported | `URLParser`, `EPUBWriter` |
| Test files | `_test.go` suffix | `scraper_test.go` |
| Mock files | `mock_*.go` | `mock_scraper.go` |

### Linting

We use [`golangci-lint`](https://golangci-lint.run/). Run it before pushing:

```bash
make lint

# Or directly
golangci-lint run ./...
```

The lint config lives in `.golangci.yml`. Do not disable linter rules in code without a comment explaining why:

```go
//nolint:errcheck // connection close errors are intentionally ignored on shutdown
conn.Close()
```

### Formatting

All code must be formatted with `gofmt` (enforced in CI):

```bash
gofmt -w .
```

Or via Make:

```bash
make fmt
```

---

## 6. Testing

### Running Tests

```bash
# All tests
go test ./...

# With race detector (required before opening a PR)
go test -race ./...

# With coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Or via Make:

```bash
make test
make test-race
```

### Test Conventions

- Unit tests live next to the code they test (`scraper_test.go` beside `scraper.go`).
- Integration tests that require network access are placed in `internal/<pkg>/integration_test.go` and guarded with a build tag:

```go
//go:build integration

package scraper_test
```

Run integration tests explicitly:

```bash
go test -tags=integration ./...
```

- Use `testdata/` for fixtures (HTML snapshots, mock API responses). Commit these files — they make tests reproducible without network access.
- Table-driven tests are preferred for functions with multiple input cases:

```go
func TestGetChapterNumber(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  int
        err   bool
    }{
        {"valid chapter", "Chapter 42", 42, false},
        {"missing number", "Prologue", 0, true},
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got, err := getChapterNumber(tc.input)
            if tc.err && err == nil {
                t.Fatal("expected error, got nil")
            }
            if got != tc.want {
                t.Errorf("got %d, want %d", got, tc.want)
            }
        })
    }
}
```

- Minimum acceptable coverage for new packages: **80%**. CI will report coverage but not block on it — reviewers may request additional tests.

### Mocking

Use interface-based mocks. Generate them with [`mockery`](https://github.com/vektra/mockery):

```bash
go generate ./...
```

Mock files follow the `mock_*.go` naming convention and are committed to the repository.

---

## 7. Adding a New Source Scraper

This is the most common contribution. Follow these steps:

### Step 1 — Create the scraper package

```
internal/scraper/<source-code>/
├── scraper.go        # Implements ScraperInterface
└── scraper_test.go   # Unit tests using testdata fixtures
```

### Step 2 — Implement the interface

```go
package mysource

import (
    "context"
    "fmt"

    "github.com/ducminhgd/manga-chef/pkg/sources"
)

type Scraper struct {
    baseURL string
    client  HTTPClient // injected for testability
}

func New(baseURL string, client HTTPClient) *Scraper {
    return &Scraper{baseURL: baseURL, client: client}
}

func (s *Scraper) GetChapterList(ctx context.Context, mangaURL string) ([]sources.Chapter, error) {
    // implementation
}

func (s *Scraper) GetImageURLs(ctx context.Context, chapterURL string) ([]string, error) {
    // implementation
}
```

### Step 3 — Register the scraper

Add your scraper to `internal/scraper/registry.go`:

```go
func init() {
    Register("mysource", func(cfg sources.SourceConfig) (ScraperInterface, error) {
        return mysource.New(cfg.BaseURL, http.DefaultClient), nil
    })
}
```

### Step 4 — Add a source YAML file

Add a ready-to-use config to `sources/`:

```yaml
# sources/mysource.yml
sources:
  - name: "My Source"
    code: "mysource"
    base_url: "https://example-manga.com"
    scraper: "mysource"
    rate_limit_ms: 300
```

### Step 5 — Add testdata fixtures

Capture a real HTML response from the source and save it under `testdata/`:

```
internal/scraper/mysource/testdata/
├── chapter_list.html     # Snapshot of the manga main page
└── chapter_page.html     # Snapshot of a single chapter page
```

Tests should use these fixtures — never make live network calls in unit tests.

### Step 6 — Document the scraper

Add a brief entry to `sources/README.md` describing the source, any known limitations, and whether authentication is required.

---

## 8. Commit Messages

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification.

### Format

```
<type>(<scope>): <short description>

[optional body]

[optional footer]
```

### Types

| Type | When to use |
|---|---|
| `feat` | A new feature |
| `fix` | A bug fix |
| `refactor` | Code change with no behavior change |
| `test` | Adding or updating tests |
| `docs` | Documentation only |
| `chore` | Build system, CI, dependencies |
| `perf` | Performance improvement |

### Examples

```
feat(scraper): add MangaDex API scraper

Implements GetChapterList and GetImageURLs against the MangaDex v5 API.
Uses API key header when provided in source config.

Closes #42
```

```
fix(downloader): retry on 503 status code

503 responses were previously treated as permanent failures.
Now retried with exponential backoff up to the configured retry limit.
```

- Keep the subject line under **72 characters**.
- Use the imperative mood: "add feature" not "added feature".
- Reference issues in the footer: `Closes #<issue>` or `Refs #<issue>`.

---

## 9. Pull Request Guidelines

### Before Opening a PR

- [ ] `go build ./...` passes cleanly
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run ./...` reports no new issues
- [ ] New public functions and types have doc comments
- [ ] `CHANGELOG.md` updated if applicable (for features and bug fixes)

### PR Description

Use this template:

```markdown
## Summary
<!-- What does this PR do? One paragraph. -->

## Changes
<!-- Bullet list of the key changes made. -->

## Testing
<!-- How was this tested? Unit tests? Manual testing? -->

## Related Issues
<!-- Closes #XX or Refs #XX -->
```

### Review Process

- All PRs require at least **one approval** from a maintainer before merging.
- CI must be green (build, lint, test).
- Maintainers may request changes. Address feedback with new commits — do not force-push while a review is in progress.
- Once approved, the PR author merges (squash merge is preferred for small PRs; merge commit for large features).

---

## 10. Reporting Issues

When filing a bug report, please include:

- **Manga Chef version:** `manga-chef --version`
- **Go version:** `go version`
- **OS and architecture:** e.g. `linux/amd64`, `darwin/arm64`
- **Source config used** (redact any personal URLs if needed)
- **Command run** and the full output with `--log-level debug`
- **Expected behavior** vs **actual behavior**

For feature requests, describe the use case first — what problem are you trying to solve? — before proposing a specific solution.

---

## Useful Make Targets

```bash
make build        # Build the binary
make test         # Run unit tests
make test-race    # Run unit tests with race detector
make lint         # Run golangci-lint
make fmt          # Format all Go files
make generate     # Run go generate (regenerate mocks)
make clean        # Remove build artifacts
```

---

*Happy contributing! If anything in this guide is unclear, open an issue or start a discussion.*