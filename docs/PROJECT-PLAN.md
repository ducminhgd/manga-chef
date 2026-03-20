# Manga Chef — Engineering Project Plan

**Language:** Go (primary)  
**First target source:** TruyenQQ (`truyenqqto.com`)  
**First sample manga:** Dấu Ấn Rồng Thiêng - Dragon Quest (349 chapters)  
**Delivery target:** MVP — fully working end-to-end download + PDF/EPUB conversion for TruyenQQ  

---

## Delivery Phases

| Phase                    | Goal                                   | Scope                                                                 |
| ------------------------ | -------------------------------------- | --------------------------------------------------------------------- |
| **Phase 0 – Foundation** | Project skeleton, tooling, CI          | Repo layout, Go modules, Makefile, linting, test harness              |
| **Phase 1 – MVP**        | Download Dấu Ấn Rồng Thiêng end-to-end | YAML config loader, TruyenQQ scraper, downloader, PDF output, CLI     |
| **Phase 2 – V1**         | Generalized, extensible platform       | Scraper registry, generic CSS scraper, EPUB/MOBI, resume, progress UI |
| **Phase 3 – Polish**     | Hardening & UX                         | Rate limiting, retry strategies, volume merge, testdata coverage      |

---

## 📦 Feature 1: Project Foundation

> Establishes the Go project skeleton, module structure, code quality tooling, and CI pipeline. Everything else builds on top of this. No business logic — but skipping or rushing it causes compounding pain.

---

### 🔷 Epic 1.1 — Repository & Module Setup

> Initialize the Go module, directory layout (matching the CONTRIBUTING.md structure), and dependency baseline.

- **Priority:** P0
- **Estimate:** S
- **Dependencies:** None

| #    | Task                                                      | Estimate | Role | Notes                                                                          |
| ---- | --------------------------------------------------------- | -------- | ---- | ------------------------------------------------------------------------------ |
| T-01 | Initialize Go module (`github.com/manga-chef/manga-chef`) | XS       | BE   | `go mod init`; set minimum Go version in `go.mod`                              |
| T-02 | Create canonical directory structure                      | XS       | BE   | `cmd/`, `internal/`, `pkg/`, `sources/`, `testdata/`, `scripts/`               |
| T-03 | Add core dependencies to `go.mod`                         | S        | BE   | `cobra`, `gopkg.in/yaml.v3`, `golang.org/x/net`, `go-epub`, `jung-kurt/gofpdf` |
| T-04 | Create root `Makefile` with standard targets              | XS       | BE   | `build`, `test`, `test-race`, `lint`, `fmt`, `clean`, `generate`               |
| T-05 | Add `sources.example.yml` with annotated TruyenQQ entry   | XS       | BE   | Serves as the reference config for contributors                                |

---

### 🔷 Epic 1.2 — Code Quality Tooling

> Linting, formatting, and static analysis configured and enforced locally and in CI.

- **Priority:** P0
- **Estimate:** S
- **Dependencies:** Epic 1.1

| #    | Task                                                          | Estimate | Role | Notes                                                                             |
| ---- | ------------------------------------------------------------- | -------- | ---- | --------------------------------------------------------------------------------- |
| T-06 | Add `.golangci.yml` with agreed linter set                    | XS       | BE   | Enable: `errcheck`, `staticcheck`, `govet`, `gofmt`, `gosimple`, `unused`         |
| T-07 | Add `mockery` config for interface mock generation            | XS       | BE   | `.mockery.yaml` targeting `internal/scraper` and `internal/downloader` interfaces |
| T-08 | Configure `gofmt` pre-commit check via `scripts/check-fmt.sh` | XS       | BE   | Fails fast before push                                                            |

---

### 🔷 Epic 1.3 — CI Pipeline (GitHub Actions)

> Automated build, lint, and test on every push and pull request.

- **Priority:** P0
- **Estimate:** S
- **Dependencies:** Epic 1.2

| #    | Task                                             | Estimate | Role   | Notes                                                          |
| ---- | ------------------------------------------------ | -------- | ------ | -------------------------------------------------------------- |
| T-09 | Add `.github/workflows/ci.yml`                   | S        | DevOps | Triggers on push + PR to `main`; jobs: `lint`, `test`, `build` |
| T-10 | Add `go test -race ./...` step to CI             | XS       | DevOps | Race detector must pass before merge                           |
| T-11 | Add coverage report step (output to job summary) | XS       | DevOps | Use `go test -coverprofile=coverage.out` + `go tool cover`     |
| T-12 | Add build matrix for Linux, macOS, Windows       | XS       | DevOps | `GOOS` / `GOARCH` matrix; artifacts uploaded per OS            |

---

## 📦 Feature 2: Source Configuration

> The YAML-based source system is the backbone of Manga Chef's extensibility. Every other feature depends on having a valid, loaded `SourceConfig` in memory.

---

### 🔷 Epic 2.1 — YAML Config Loader & Validator

> Parse `sources.yml`, validate the schema, and expose a typed `SourceConfig` struct to the rest of the application.

- **Priority:** P0
- **Estimate:** M
- **Dependencies:** Epic 1.1

| #    | Task                                                                 | Estimate | Role | Notes                                                                                            |
| ---- | -------------------------------------------------------------------- | -------- | ---- | ------------------------------------------------------------------------------------------------ |
| T-13 | Define `pkg/sources/types.go` — public domain types                  | S        | BE   | `SourceConfig`, `Chapter`, `Page` structs                                                        |
| T-14 | Implement `internal/config/loader.go` — YAML → `[]SourceConfig`      | S        | BE   | Reads from file path; supports single file and `sources/` directory discovery                    |
| T-15 | Implement schema validation with line-specific error messages        | S        | BE   | Required fields: `name`, `code`, `base_url`, `scraper`. Report all errors at once, not fail-fast |
| T-16 | Write unit tests for loader (valid config, missing fields, bad YAML) | S        | BE   | Use `testdata/config/` fixtures                                                                  |
| T-17 | Implement `sources list` CLI sub-command                             | XS       | BE   | Prints table: code, name, base_url, scraper                                                      |
| T-18 | Implement `sources add <file>` CLI sub-command                       | S        | BE   | Validates then merges into active config; detects duplicate `code`                               |

**Sub-tasks for T-15:**
- T-15.1: Validate `code` is lowercase alphanumeric/underscore (no spaces)
- T-15.2: Validate `base_url` is a parseable URL
- T-15.3: Validate `scraper` references a known built-in name or an existing file path
- T-15.4: Warn (not error) if `rate_limit_ms` is 0 for HTML scraper sources

---

### 🔷 Epic 2.2 — TruyenQQ Source YAML

> Ship the production-ready TruyenQQ source config as a bundled `sources/truyenqq.yml`.

- **Priority:** P0
- **Estimate:** S
- **Dependencies:** Epic 2.1, Epic 3.1 (scraper must exist to validate)

| #    | Task                                                                           | Estimate | Role | Notes                                                                             |
| ---- | ------------------------------------------------------------------------------ | -------- | ---- | --------------------------------------------------------------------------------- |
| T-19 | Author `sources/truyenqq.yml` with correct `base_url`, headers, and rate limit | XS       | BE   | `Referer` header required; `rate_limit_ms: 500` recommended                       |
| T-20 | Document TruyenQQ URL pattern in `sources/README.md`                           | XS       | Docs | Manga page: `/truyen-tranh/<slug>`, chapter: `/truyen-tranh/<slug>-chap-<n>.html` |

---

## 📦 Feature 3: Scraper Engine

> The scraper engine defines the interface contract, the registry, and the first concrete implementation: TruyenQQ. This is the most technically risky feature — the HTML structure of third-party sites can be fragile.

---

### 🔷 Epic 3.1 — Scraper Interface & Registry

> Define the `ScraperInterface` in Go, implement the registry (map from source `code` to factory function), and scaffold the test harness.

- **Priority:** P0
- **Estimate:** M
- **Dependencies:** Epic 2.1

| #    | Task                                                                            | Estimate | Role | Notes                                                                                                          |
| ---- | ------------------------------------------------------------------------------- | -------- | ---- | -------------------------------------------------------------------------------------------------------------- |
| T-21 | Define `internal/scraper/scraper.go` — `ScraperInterface`                       | S        | BE   | Methods: `GetChapterList(ctx, mangaURL) ([]Chapter, error)`, `GetImageURLs(ctx, chapterURL) ([]string, error)` |
| T-22 | Implement `internal/scraper/registry.go` — factory map + `Register()` / `Get()` | S        | BE   | Thread-safe; `init()` pattern for built-ins; returns `ErrScraperNotFound`                                      |
| T-23 | Define `HTTPClient` interface in `internal/scraper/` (for test injection)       | XS       | BE   | Wraps `http.Client`; enables mock injection in all scraper tests                                               |
| T-24 | Generate mock for `ScraperInterface` via `go generate`                          | XS       | BE   | Used by downloader tests                                                                                       |

---

### 🔷 Epic 3.2 — TruyenQQ Scraper Implementation

> Implement the `truyenqq` scraper that parses the HTML chapter list page and each chapter's image list. This epic is the hands-on reverse-engineering work against the live site.

- **Priority:** P0
- **Estimate:** L
- **Dependencies:** Epic 3.1

**TruyenQQ URL patterns (verified):**
- Manga main page: `https://truyenqqto.com/truyen-tranh/dau-an-rong-thieng-236`
- Chapter page: `https://truyenqqto.com/truyen-tranh/dau-an-rong-thieng-236-chap-1.html`

| #    | Task                                                                              | Estimate | Role | Notes                                                                                |
| ---- | --------------------------------------------------------------------------------- | -------- | ---- | ------------------------------------------------------------------------------------ |
| T-25 | Capture and commit HTML fixtures for TruyenQQ                                     | S        | BE   | Save manga main page + one chapter page HTML to `testdata/truyenqq/`                 |
| T-26 | Implement `GetChapterList()` — parse chapter list from manga main page            | M        | BE   | CSS selector: identify `ul.list-chapter li a`; extract chapter number + title + URL  |
| T-27 | Implement `GetImageURLs()` — parse image URLs from chapter reader page            | M        | BE   | CSS selector: identify `div.page-chapter img`; handle `data-src` lazy-load attribute |
| T-28 | Handle chapter number extraction from Vietnamese text (`"Chap 1"`, `"Chapter 1"`) | S        | BE   | Regex + fallback; normalize to float64 for correct sort order                        |
| T-29 | Respect `Referer` and `User-Agent` headers (required by TruyenQQ CDN)             | S        | BE   | Inject from `SourceConfig.Headers` into every request                                |
| T-30 | Write unit tests for `GetChapterList()` using HTML fixture                        | S        | BE   | Assert count, first/last chapter number, chapter URL format                          |
| T-31 | Write unit tests for `GetImageURLs()` using HTML fixture                          | S        | BE   | Assert image count, URL validity, order                                              |
| T-32 | Write integration test (`//go:build integration`) against live site               | S        | BE   | Downloads chapter 1 of Dấu Ấn Rồng Thiêng; not run in CI by default                  |

**Sub-tasks for T-26:**
- T-26.1: Inspect live HTML — identify the chapter list container selector
- T-26.2: Handle reverse-ordered chapter list (TruyenQQ lists newest first)
- T-26.3: Handle pagination if chapter list spans multiple pages

**Sub-tasks for T-27:**
- T-27.1: Inspect live chapter HTML — identify image container selector
- T-27.2: Handle CDN URLs that require `Referer` header to return 200 (not 403)
- T-27.3: Handle `data-src` vs `src` attribute (lazy loading pattern)

---

### 🔷 Epic 3.3 — Generic CSS Selector Scraper

> A zero-code fallback scraper driven entirely by CSS selectors declared in the YAML config. Enables simple HTML sources to be added without writing Go.

- **Priority:** P1
- **Estimate:** M
- **Dependencies:** Epic 3.1

| #    | Task                                                               | Estimate | Role | Notes                                                                                  |
| ---- | ------------------------------------------------------------------ | -------- | ---- | -------------------------------------------------------------------------------------- |
| T-33 | Implement `internal/scraper/generic/scraper.go`                    | M        | BE   | Reads `selectors` block from `SourceConfig`; uses `golang.org/x/net/html` or `goquery` |
| T-34 | Support `attr:` prefix in selector values (e.g., `attr:data-src`)  | S        | BE   | Extract attribute value instead of text content                                        |
| T-35 | Write unit tests for generic scraper using synthetic HTML fixtures | S        | BE   | Cover `src`, `href`, `data-src` attribute extraction cases                             |

---

## 📦 Feature 4: Image Downloader

> The concurrent download engine. Takes a list of image URLs from the scraper and saves them to the local filesystem in page order. Reliability (retry, resume) is more important than raw speed.

---

### 🔷 Epic 4.1 — Core Download Engine

> Concurrent worker pool for downloading images with configurable parallelism.

- **Priority:** P0
- **Estimate:** L
- **Dependencies:** Epic 3.1

| #    | Task                                                                     | Estimate | Role | Notes                                                                     |
| ---- | ------------------------------------------------------------------------ | -------- | ---- | ------------------------------------------------------------------------- |
| T-36 | Implement `internal/downloader/downloader.go` — `Downloader` struct      | S        | BE   | Accepts `ScraperInterface` (injected); `context.Context`-aware throughout |
| T-37 | Implement worker pool with `errgroup` + goroutines                       | M        | BE   | `--workers` flag (default 4); semaphore pattern via buffered channel      |
| T-38 | Implement deterministic output path: `<out>/<title>/<chapter>/NNN.ext`   | S        | BE   | Zero-pad page numbers to 3 digits; preserve original file extension       |
| T-39 | Implement skip logic: skip chapter if folder exists + file count matches | S        | BE   | Compare expected count (from scraper) vs actual files on disk             |
| T-40 | Implement `--force` flag to re-download existing chapters                | XS       | BE   | Deletes existing chapter folder before downloading                        |
| T-41 | Write unit tests using mock scraper and in-memory HTTP server            | M        | BE   | Test: happy path, partial chapter (resume), 404 handling                  |

---

### 🔷 Epic 4.2 — Retry & Resilience

> Exponential backoff retry for transient failures. Resume partial chapter downloads.

- **Priority:** P0
- **Estimate:** M
- **Dependencies:** Epic 4.1

| #    | Task                                                                                   | Estimate | Role | Notes                                                                                           |
| ---- | -------------------------------------------------------------------------------------- | -------- | ---- | ----------------------------------------------------------------------------------------------- |
| T-42 | Implement `internal/downloader/retry.go` — exponential backoff with jitter             | S        | BE   | Max retries configurable via `SourceConfig.Retries` (default 3); respect `context` cancellation |
| T-43 | Retry on: network timeout, 5xx response, connection reset                              | XS       | BE   | Do NOT retry on 403/404 (permanent failures — log and skip)                                     |
| T-44 | Implement resume: scan existing files in chapter folder, skip already-downloaded pages | S        | BE   | Compare page index (filename) vs total from scraper                                             |
| T-45 | Write tests for retry logic using mock HTTP server that fails N times                  | S        | BE   | Assert: eventual success, context cancellation stops retries, 404 skips                         |

---

### 🔷 Epic 4.3 — Progress Reporting

> Per-chapter progress display in the terminal during download.

- **Priority:** P1
- **Estimate:** S
- **Dependencies:** Epic 4.1

| #    | Task                                                     | Estimate | Role | Notes                                                                          |
| ---- | -------------------------------------------------------- | -------- | ---- | ------------------------------------------------------------------------------ |
| T-46 | Implement progress reporter interface                    | XS       | BE   | `OnStart(total int)`, `OnProgress(done int)`, `OnDone()`, `OnError(err error)` |
| T-47 | Implement terminal progress bar (no external dependency) | S        | BE   | Simple `\r` overwrite pattern; shows: chapter name, N/total images, %          |
| T-48 | Implement `--quiet` flag to suppress progress output     | XS       | BE   | Useful for scripting / CI usage                                                |

---

## 📦 Feature 5: Format Conversion

> Converts downloaded image folders into PDF, EPUB, or MOBI files. PDF is required for MVP; EPUB for V1.

---

### 🔷 Epic 5.1 — PDF Conversion

> Convert a chapter's image folder into a single PDF. Required for the MVP end-to-end demo.

- **Priority:** P0
- **Estimate:** M
- **Dependencies:** Epic 4.1

| #    | Task                                                                                     | Estimate | Role | Notes                                                                        |
| ---- | ---------------------------------------------------------------------------------------- | -------- | ---- | ---------------------------------------------------------------------------- |
| T-49 | Define `internal/converter/converter.go` — `ConverterInterface`                          | XS       | BE   | `Convert(ctx, inputDir, outputPath string, opts Options) error`              |
| T-50 | Implement `internal/converter/pdf/pdf.go` using `gofpdf`                                 | M        | BE   | One image per page; preserve aspect ratio; letter-size page                  |
| T-51 | Handle JPEG, PNG, WebP input formats                                                     | S        | BE   | Convert WebP to PNG before embedding if `gofpdf` doesn't support it natively |
| T-52 | Implement `convert` CLI sub-command                                                      | S        | BE   | Supports chapter-dir conversion and root-dir volume conversion (`--max-*` limits) |
| T-53 | Write tests: convert 3-image fixture folder to PDF, verify output file exists + size > 0 | S        | BE   | Golden file test optional; existence + non-zero size is sufficient for CI    |

---

### 🔷 Epic 5.2 — EPUB Conversion

> Convert a chapter's image folder to a valid EPUB file with metadata.

- **Priority:** P1
- **Estimate:** M
- **Dependencies:** Epic 5.1 (converter interface established)

| #    | Task                                                                        | Estimate | Role | Notes                                                               |
| ---- | --------------------------------------------------------------------------- | -------- | ---- | ------------------------------------------------------------------- |
| T-54 | Implement `internal/converter/epub/epub.go` using `go-epub`                 | M        | BE   | Embed images as EPUB image items; one image per "page" HTML file    |
| T-55 | Add metadata: title, chapter number, cover image (first image)              | S        | BE   | Extract manga title + chapter number from directory path convention |
| T-56 | Write tests: 3-image fixture → EPUB; validate with `epubcheck` if available | S        | BE   |                                                                     |

---

### 🔷 Epic 5.3 — MOBI Conversion

> Generate MOBI via Calibre `ebook-convert`. Requires Calibre installed on the host.

- **Priority:** P2
- **Estimate:** S
- **Dependencies:** Epic 5.2 (convert EPUB first, then to MOBI)

| #    | Task                                                                       | Estimate | Role | Notes                                                  |
| ---- | -------------------------------------------------------------------------- | -------- | ---- | ------------------------------------------------------ |
| T-57 | Implement `internal/converter/mobi/mobi.go` — shell out to `ebook-convert` | S        | BE   | EPUB → MOBI via `ebook-convert input.epub output.mobi` |
| T-58 | Detect Calibre availability at startup; skip MOBI gracefully if not found  | XS       | BE   | Clear error message with installation instructions     |
| T-59 | Write test: mock exec call; verify correct CLI args assembled              | XS       | BE   |                                                        |

---

### 🔷 Epic 5.4 — Volume Merge

> Merge chapter directories into one or more volume directories, each respecting three
> independent limits: maximum file size, maximum page count, and maximum chapter count.
> Chapters are always kept whole — never split across volumes. The first limit breached
> closes the current volume, regardless of which limit triggered.

**Implemented behavior update:**
- `convert --input <manga-root-dir>` now auto-plans volumes using these same limits and converts each planned volume.
- `merge --input <manga-root-dir>` creates merged directories and optionally converts them.
- Volume directory naming: `VOL_<VolumeSequence>_C<FromChapter>-C<ToChapter>`.
- Optional cleanup: `--delete-merged-chapters` removes source chapter directories after a successful merge.
- Optional conversion: `--convert pdf|epub|mobi` generates output files per merged volume.

**Volume planning rules:**
1. Chapters are packed greedily in order. Before appending a chapter, check all enabled limits against the post-addition totals.
2. If any limit would be exceeded **and** the current volume is non-empty, flush the current volume and start a new one with that chapter.
3. If a single chapter on its own exceeds a limit (e.g. a 600-page chapter with `--max-pages 500`), it is placed alone in its own volume and a warning is emitted. It is never skipped.
4. All three limits are checked simultaneously — the first one breached wins.

**Limits and defaults:**

| Limit                         | Flag             | Default | Disable |
| ----------------------------- | ---------------- | ------- | ------- |
| Output file size              | `--max-size-mb`  | 200 MB  | `-1`    |
| Total pages per volume        | `--max-pages`    | 500     | `-1`    |
| Number of chapters per volume | `--max-chapters` | 30      | `-1`    |

**Example** (from spec): `--max-pages 500`, each chapter has 18 pages.
- Chapters 1–27 → 486 pages ≤ 500. Adding ch. 28 would make 504 > 500 → close volume 1.
- Chapter 28 → volume 2.

- **Priority:** P2
- **Estimate:** L
- **Dependencies:** Epic 5.1, Epic 5.2

| #    | Task                                                                                       | Estimate | Role | Notes                                                                                                                                    |
| ---- | ------------------------------------------------------------------------------------------ | -------- | ---- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| T-60 | Implement merge limits model in CLI flow (`internal/cli/convert_volume.go`)               | S        | BE   | Fields: `MaxFileSizeMB`, `MaxPages`, `MaxChapters`; zero = use default, negative = disable                                             |
| T-61 | Implement volume planner (`planVolumes`) with greedy packing                               | M        | BE   | Returns `[]volumePlan` with chapter slice, total pages, total bytes                                                                      |
| T-62 | Handle oversized single chapters — place alone, emit warning                               | S        | BE   | Warning printed to CLI output                                                                                                             |
| T-63 | Implement chapter directory discovery + ordering from root input                           | S        | BE   | Supports chapter folder patterns such as `chap-001`; falls back to lexical ordering when number parse fails                             |
| T-64 | Implement `merge` CLI sub-command                                                          | M        | BE   | Flags: `--input`, `--output`, `--max-size-mb`, `--max-pages`, `--max-chapters`, `--delete-merged-chapters`, `--convert`, `--title`    |
| T-65 | Implement merged directory naming convention                                                | S        | BE   | `VOL_<VolumeSequence>_C<FromChapter>-C<ToChapter>`                                                                                        |
| T-66 | Add tests for planner + merge command behaviors                                             | M        | BE   | Covers split boundaries, directory naming, delete option, and convert option                                                              |
| T-67 | Integrate root-directory conversion behavior into `convert` command                         | S        | BE   | `convert` auto-detects chapter dir vs manga root dir and applies volume planning for root input                                          |

**Sub-tasks for T-61 (implemented):**
- T-61.1: `would-exceed` check evaluated before append (not after) — ensures chapters stay whole
- T-61.2: OR logic across all three limits — any single breach triggers the flush
- T-61.3: Volume struct carries `Index` (1-based), `Chapters`, `TotalPages`, `TotalBytes`
- T-61.4: Warnings are surfaced via CLI output

**Sub-tasks for T-64 (implemented):**
- T-64.1: Merge all discovered chapter directories from `--input`
- T-64.2: Materialize merged images in each volume directory in deterministic page order
- T-64.3: Optional converter invocation (PDF/EPUB/MOBI) once per volume
- T-64.4: Output files for converted volumes use the merged volume basename (e.g. `VOL_001_C1-C5.pdf`)

---

## 📦 Feature 6: CLI

> The user-facing cobra command tree. Wires together all internal packages. No business logic lives here.

---

### 🔷 Epic 6.1 — CLI Command Tree

> Full cobra command tree covering all user-facing operations.

- **Priority:** P0
- **Estimate:** M
- **Dependencies:** All internal packages (depends on Epics 2–5)

| #    | Task                                                               | Estimate | Role | Notes                                                                            |
| ---- | ------------------------------------------------------------------ | -------- | ---- | -------------------------------------------------------------------------------- |
| T-63 | Scaffold `cmd/manga-chef/main.go` and root cobra command           | XS       | BE   | Sets version (`--version`), global flags: `--sources`, `--output`, `--log-level` |
| T-64 | Implement `download` sub-command                                   | M        | BE   | Flags: `--source`, `--url`, `--chapters`, `--convert`, `--workers`, `--force`    |
| T-65 | Implement `chapters` sub-command (list without downloading)        | S        | BE   | Flags: `--source`, `--url`; outputs table: #, title, URL                         |
| T-66 | Implement `sources list` and `sources add` sub-commands            | S        | BE   | See Epic 2.1                                                                     |
| T-67 | Implement `convert` sub-command                                    | S        | BE   | Supports chapter-directory conversion and root-directory volume conversion        |
| T-68 | Implement `merge` sub-command                                      | S        | BE   | Creates `VOL_*` directories; supports optional delete + optional format conversion |
| T-69 | Add `--log-level` global flag (debug / info / warn / error)        | XS       | BE   | Structured JSON logging via `log/slog` (stdlib, Go 1.21+)                        |
| T-70 | Write integration test: full `download` flow with mock HTTP server | M        | BE   | Uses `httptest.Server`; verifies files created on disk in correct structure      |

---

## 📦 Feature 7: End-to-End Demo (Dấu Ấn Rồng Thiêng)

> Validate the full stack against the real TruyenQQ site using the sample manga. This is the acceptance gate for MVP.

---

### 🔷 Epic 7.1 — MVP Acceptance Run

> Run the full download + convert pipeline for Dấu Ấn Rồng Thiêng ch.1–5 against truyenqqto.com.

- **Priority:** P0
- **Estimate:** M
- **Dependencies:** All Phase 1 epics

| #    | Task                                                                                               | Estimate | Role | Notes                                                                          |
| ---- | -------------------------------------------------------------------------------------------------- | -------- | ---- | ------------------------------------------------------------------------------ |
| T-71 | Prepare `sources/truyenqq.yml` pointing to `truyenqqto.com`                                        | XS       | BE   | Validated against live site                                                    |
| T-72 | Manual run: `manga-chef download --source truyenqq --url <manga-url> --chapters 1-5 --convert pdf` | S        | BE   | Verify: 5 chapter folders created, 5 PDFs generated, correct page order        |
| T-73 | Verify PDF opens correctly in a PDF viewer                                                         | XS       | QA   | Check first and last page of ch.1; confirm no corrupt images                   |
| T-74 | Document the end-to-end command in `README.md` with TruyenQQ example                               | S        | Docs | Include: install, config, download, convert — working example using this manga |
| T-75 | Capture any TruyenQQ-specific quirks in `sources/README.md`                                        | XS       | Docs | e.g., CDN `Referer` requirement, rate limit sensitivity                        |

---

## Dependency Map

```
Epic 1.1 (Repo Setup)
  └── Epic 1.2 (Tooling)
        └── Epic 1.3 (CI)

Epic 2.1 (Config Loader)        ← unblocks everything below
  ├── Epic 2.2 (TruyenQQ YAML)
  └── Epic 3.1 (Scraper Interface)
        ├── Epic 3.2 (TruyenQQ Scraper)    ← critical path
        ├── Epic 3.3 (Generic Scraper)      ← parallel, P1
        └── Epic 4.1 (Downloader)
              ├── Epic 4.2 (Retry)          ← parallel with 4.3
              ├── Epic 4.3 (Progress)       ← parallel with 4.2
              └── Epic 5.1 (PDF)            ← critical path
                    ├── Epic 5.2 (EPUB)     ← parallel, P1
                    ├── Epic 5.3 (MOBI)     ← parallel, P2
                    └── Epic 5.4 (Merge)    ← parallel, P2

Epic 6.1 (CLI)                  ← wires all above
  └── Epic 7.1 (MVP Demo)       ← acceptance gate
```

**Critical path (MVP):**
`1.1 → 1.2 → 1.3 → 2.1 → 3.1 → 3.2 → 4.1 → 4.2 → 5.1 → 6.1 → 7.1`

**Parallel workstreams (after Epic 3.1 is done):**
- Scraper (3.2) and Downloader (4.1) can be developed in parallel once the interface is locked
- EPUB/MOBI/Merge (5.2–5.4) can proceed in parallel with CLI wiring (6.1)
- Generic scraper (3.3) is fully independent after 3.1 — good for a second contributor

---

## 📊 Summary

| Metric                   | Value                                      |
| ------------------------ | ------------------------------------------ |
| Total Features           | 7                                          |
| Total Epics              | 18                                         |
| Total Tasks              | 80                                         |
| **Phase 1 (MVP) Tasks**  | **~45 tasks** (P0 only)                    |
| **Phase 2 (V1) Tasks**   | **~30 tasks** (P1 + P2)                    |
| Estimated Effort (MVP)   | ~6–8 person-weeks                          |
| Estimated Effort (V1)    | ~10–14 person-weeks total                  |
| Suggested Team Size      | 1–2 engineers                              |
| Recommended MVP Timeline | 6–8 weeks (solo) / 3–4 weeks (2 engineers) |

---

## ⚠️ Key Risks & Assumptions

| #    | Risk                                                              | Likelihood | Impact | Mitigation                                                                       |
| ---- | ----------------------------------------------------------------- | ---------- | ------ | -------------------------------------------------------------------------------- |
| R-01 | TruyenQQ changes its HTML structure (anti-scraping update)        | Medium     | High   | Commit HTML fixtures to `testdata/`; integration tests will catch breakage early |
| R-02 | TruyenQQ CDN blocks requests without valid `Referer` / User-Agent | High       | High   | Already accounted for in T-29; verify in T-72 before tagging MVP                 |
| R-03 | TruyenQQ rate-limits or bans IPs for aggressive crawling          | Medium     | Medium | Default `rate_limit_ms: 500`; configurable; test with low `--workers` first      |
| R-04 | `gofpdf` doesn't support WebP images (common on newer sources)    | Medium     | Medium | T-51 explicitly handles WebP → PNG conversion as a pre-processing step           |
| R-05 | Chapter list pagination on TruyenQQ is undiscovered               | Low        | High   | Sub-task T-26.3 explicitly investigates this during scraper development          |

**Assumptions:**
- TruyenQQ image CDN URLs are direct HTTP links (not JavaScript-rendered or token-protected)
- Dấu Ấn Rồng Thiêng (349 chapters) is fully available on `truyenqqto.com`
- No Cloudflare challenge page protection on TruyenQQ (standard Go `http.Client` is sufficient)
- Developer has Go 1.22+ installed locally

---

## 🔜 Suggested Next Steps

1. **Start with Epic 1.1–1.3** (Foundation) — takes less than a day and pays off immediately
2. **Reverse-engineer TruyenQQ HTML in parallel** (T-25–T-27 sub-tasks) — open the manga page in browser DevTools, capture the chapter list and reader page selectors, commit the fixtures before writing any Go
3. **Lock the `ScraperInterface` early** (Epic 3.1) — everything downstream depends on it; a wrong interface costs significant rework
4. **Aim for T-72 as the first real milestone** — the moment `manga-chef download` produces 5 real PDFs of Dấu Ấn Rồng Thiêng is the MVP
5. **Track tasks in GitHub Issues** — one issue per Task (T-XX), label by phase and priority
