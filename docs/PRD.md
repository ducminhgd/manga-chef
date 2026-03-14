# Product Requirements Document - Manga Chef

**Version:** 1.0  
**Status:** Finalized  
**Last Updated:** 2026-03-14

---

## Table of Contents

- [Product Requirements Document - Manga Chef](#product-requirements-document---manga-chef)
  - [Table of Contents](#table-of-contents)
  - [1. Overview](#1-overview)
  - [2. Target Users](#2-target-users)
  - [3. User Stories](#3-user-stories)
  - [4. Functional Requirements](#4-functional-requirements)
    - [FR-01 — Source Configuration (YAML)](#fr-01--source-configuration-yaml)
    - [FR-02 — Chapter Discovery](#fr-02--chapter-discovery)
    - [FR-03 — Image Downloading](#fr-03--image-downloading)
    - [FR-04 — Format Conversion](#fr-04--format-conversion)
    - [FR-05 — Scraper Extensibility](#fr-05--scraper-extensibility)
    - [FR-06 — CLI Interface](#fr-06--cli-interface)
  - [5. Non-Functional Requirements](#5-non-functional-requirements)
  - [6. Source File Schema](#6-source-file-schema)
  - [7. Output Structure](#7-output-structure)
  - [8. Recommended Tech Stack](#8-recommended-tech-stack)
    - [Option A — Go CLI (Recommended for distribution)](#option-a--go-cli-recommended-for-distribution)
    - [Option B — Python CLI (Faster to prototype)](#option-b--python-cli-faster-to-prototype)
  - [9. Out of Scope (v1)](#9-out-of-scope-v1)
  - [10. Success Metrics](#10-success-metrics)
  - [11. Open Questions](#11-open-questions)

---

## 1. Overview

| Field                 | Details                                                                                                                                                                                                                               |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Product Name**      | Manga Chef                                                                                                                                                                                                                            |
| **Problem Statement** | Manga readers who want to archive or read offline have no reliable, source-agnostic tool that handles downloading, chapter organization, and format conversion from a single CLI — without being locked into one website's structure. |
| **Goal**              | A developer-friendly, extensible manga downloader where sources are declarative YAML configs and conversion to portable e-reader formats (PDF, EPUB, MOBI) is a first-class citizen.                                                  |

---

## 2. Target Users

| Persona       | Description                                                                                                                   |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| **Primary**   | Tech-savvy manga reader / developer who self-hosts their media library (Calibre user, Plex-for-books mindset, offline-first). |
| **Secondary** | Manga archivist or translator who needs bulk chapter acquisition and format portability across e-reader devices.              |

---

## 3. User Stories

- As a **user**, I want to define a manga source in YAML so that I can add or change sources without modifying code.
- As a **user**, I want to list all chapters of a manga from its main page so that I can select which ones to download.
- As a **user**, I want to download a chapter's images in the correct page order so that reading order is preserved.
- As a **user**, I want to merge downloaded images into a PDF, EPUB, or MOBI file per chapter (or per volume) so that I can load them onto my e-reader.
- As a **user**, I want to swap the base URL of a source in the YAML file without touching any logic so that I can follow mirror sites when the original goes down.
- As a **developer**, I want to write a custom scraper plugin for a source so that non-standard or JavaScript-heavy sites can still be supported.
- As a **user**, I want interrupted downloads to resume from where they stopped so that I don't re-download images I already have.

---

## 4. Functional Requirements

### FR-01 — Source Configuration (YAML)

Sources are declared in `.yml` files. Each source entry maps a human-readable name and a short `code` to a scraper implementation, with the `base_url` being the only field that changes between the original site and its mirrors.

**Example source file:**

```yaml
sources:
  - name: "MangaDex"
    code: "mangadex"
    base_url: "https://mangadex.org"
    scraper: "mangadex"           # maps to a built-in or plugin scraper

  - name: "NetTruyen"
    code: "nettruyen"
    base_url: "https://nettruyen.com"
    scraper: "nettruyen"
    headers:                      # optional HTTP headers sent with every request
      Referer: "https://nettruyen.com"
      User-Agent: "Mozilla/5.0"
    rate_limit_ms: 500            # delay (ms) between sequential requests
    retries: 5                    # override default retry count

  - name: "Custom Mirror"
    code: "custom_mirror"
    base_url: "https://mirror-site.org"
    scraper: "./scrapers/custom_scraper.py"   # path to a plugin scraper file
```

| Requirement | Description                                                                                                                          |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| FR-01.1     | `base_url` must be changeable without redeploying or editing scraper code.                                                           |
| FR-01.2     | `scraper` maps to a built-in handler by name, or to a plugin file by path. If omitted, a generic CSS-selector-based scraper is used. |
| FR-01.3     | `headers` and `rate_limit_ms` are optional; when present they are applied to all requests for that source.                           |
| FR-01.4     | Multiple sources may be declared in a single YAML file or discovered from a `sources/` directory.                                    |
| FR-01.5     | The application validates the YAML schema on startup and reports clear, line-specific errors for malformed configs.                  |

---

### FR-02 — Chapter Discovery

| Requirement | Description                                                                                                                  |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------- |
| FR-02.1     | Given a manga's main page URL and a source `code`, the app fetches and displays all available chapters (title, number, URL). |
| FR-02.2     | The chapter list is sortable (ascending/descending by chapter number).                                                       |
| FR-02.3     | Users can select a range (`--chapters 1-50`), specific chapters (`--chapters 10,15,20`), or all chapters (`--chapters all`). |
| FR-02.4     | Chapter metadata (title, number, date if available) is cached locally to avoid repeated fetches.                             |

---

### FR-03 — Image Downloading

| Requirement | Description                                                                                                                       |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------- |
| FR-03.1     | For each selected chapter, the app fetches all image URLs in correct page order as determined by the scraper.                     |
| FR-03.2     | Images are downloaded concurrently with configurable parallelism (`--workers`, default: 4).                                       |
| FR-03.3     | Images are saved under a deterministic folder structure: `<output_dir>/<manga_title>/<chapter_number>/001.jpg`.                   |
| FR-03.4     | If a chapter folder already exists with the expected file count, the chapter is skipped by default. Use `--force` to re-download. |
| FR-03.5     | Failed image downloads are retried up to N times (configurable via YAML or `--retries`, default: 3) with exponential backoff.     |
| FR-03.6     | An interrupted download session can be resumed; already-downloaded images in a partial chapter are not re-fetched.                |
| FR-03.7     | Download progress is shown per chapter (image count, percentage, estimated time remaining).                                       |

---

### FR-04 — Format Conversion

| Requirement | Description                                                                                                                                                         |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| FR-04.1     | Convert a chapter's local image folder into one of: **PDF**, **EPUB**, or **MOBI**.                                                                                 |
| FR-04.2     | Conversion can be triggered automatically after download (`--convert pdf`) or as a standalone `convert` sub-command.                                                |
| FR-04.3     | Multiple chapters can be merged into a single output file by specifying a range (`--merge --chapters 1-10 --output naruto-vol1.epub`).                              |
| FR-04.4     | EPUB output includes metadata: manga title, chapter/volume number, and cover image (first image of the chapter/volume).                                             |
| FR-04.5     | MOBI output is generated by invoking Calibre's `ebook-convert` CLI if available; if not found, the app exits with a clear error and installation instructions.      |
| FR-04.6     | Images are resized or resampled for the target format only when the user explicitly requests it (`--resize 1080x1920`); otherwise the originals are embedded as-is. |

---

### FR-05 — Scraper Extensibility

A **scraper** is a module that implements a standard two-method interface. Built-in scrapers are shipped for common sources; custom scrapers are loaded from the file path specified in the YAML config.

**Interface contract (Go example):**

```go
// Chapter represents a single manga chapter with its metadata.
type Chapter struct {
    Number float64
    Title  string
    URL    string
}

// Scraper is the interface that every source scraper must implement.
type Scraper interface {
    // GetChapterList returns all chapters on the manga's main page.
    GetChapterList(mangaURL string) ([]Chapter, error)

    // GetImageURLs returns all image URLs for a chapter, in page order.
    GetImageURLs(chapterURL string) ([]string, error)
}
```

| Requirement | Description                                                                                                          |
| ----------- | -------------------------------------------------------------------------------------------------------------------- |
| FR-05.1     | Each scraper implements `get_chapter_list(url)` and `get_image_urls(chapter_url)`.                                   |
| FR-05.2     | Built-in scrapers ship for: MangaDex (API-based), and a generic HTML scraper configurable via CSS selectors in YAML. |
| FR-05.3     | The `scraper` field in YAML can point to a file path for fully custom plugin logic.                                  |
| FR-05.4     | A generic scraper supporting CSS selector configuration is available as a no-code fallback for simple HTML sources.  |

**Generic scraper CSS selector config example:**

```yaml
- name: "Simple Site"
  code: "simplesite"
  base_url: "https://example-manga.com"
  scraper: "generic"
  selectors:
    chapter_list: "ul.chapter-list a"
    chapter_number: "attr:data-chapter"
    image_list: "div.reader-container img"
    image_url: "attr:src"
```

---

### FR-06 — CLI Interface

Manga Chef exposes a single binary with the following sub-commands:

```
# Download chapters from a source
manga-chef download \
  --source mangadex \
  --url https://mangadex.org/title/<id> \
  --chapters 1-50 \
  --output ./library \
  --convert epub \
  --workers 4

# List all chapters without downloading
manga-chef chapters \
  --source nettruyen \
  --url https://nettruyen.com/manga/<slug>

# List configured sources
manga-chef sources list

# Add a source from a YAML file
manga-chef sources add ./my_source.yml

# Convert already-downloaded images to a format
manga-chef convert \
  --input ./library/Naruto/ch1 \
  --format pdf

# Merge multiple chapters into one file
manga-chef merge \
  --input ./library/Naruto \
  --chapters 1-10 \
  --format epub \
  --output ./library/Naruto-Vol1.epub
```

---

## 5. Non-Functional Requirements

| Concern           | Requirement                                                                                               |
| ----------------- | --------------------------------------------------------------------------------------------------------- |
| **Performance**   | Download 50 images per chapter in under 30 seconds on a standard broadband connection with 4 workers.     |
| **Reliability**   | Auto-retry on transient network errors; resume interrupted downloads without data loss.                   |
| **Portability**   | Runs on Linux, macOS, and Windows. Distributed as a single self-contained binary when possible.           |
| **Extensibility** | New sources are added via YAML + optional plugin with zero changes to core application code.              |
| **Config Safety** | YAML validation on startup with clear, line-specific error messages for malformed or missing fields.      |
| **Storage**       | Output directory structure is deterministic, human-readable, and stable across versions.                  |
| **Observability** | Structured log output (JSON or plain) configurable via `--log-level`. Progress bars for active downloads. |

---

## 6. Source File Schema

Full annotated schema for a `sources.yml` file:

```yaml
# manga-chef sources configuration
# Place in: ~/.config/manga-chef/sources.yml
# Or pass with: --sources ./my-sources.yml

sources:
  - name: string           # Human-readable display name
    code: string           # Short unique identifier used in CLI (e.g. --source mangadex)
    base_url: string       # Root URL of the site; change this to switch mirrors
    scraper: string        # Built-in name OR relative path to plugin file
    enabled: bool          # Optional. Default: true
    headers:               # Optional. Extra HTTP headers for all requests
      key: value
    rate_limit_ms: int     # Optional. Delay between requests in ms. Default: 0
    retries: int           # Optional. Max retry attempts per image. Default: 3
    timeout_s: int         # Optional. Request timeout in seconds. Default: 30
    selectors:             # Optional. Only used when scraper: "generic"
      chapter_list: string
      chapter_number: string
      image_list: string
      image_url: string
```

---

## 7. Output Structure

```
<output_dir>/
└── <Manga Title>/
    ├── cover.jpg                     # Cover image (if available)
    ├── ch001/
    │   ├── 001.jpg
    │   ├── 002.jpg
    │   └── ...
    ├── ch002/
    │   └── ...
    ├── Manga-Title-ch001.pdf         # Converted output (if --convert used)
    ├── Manga-Title-ch001.epub
    └── Manga-Title-Vol1.epub         # Merged output (if --merge used)
```

---

## 8. Recommended Tech Stack

Two viable approaches given the Go + Python background:

### Option A — Go CLI (Recommended for distribution)

| Layer           | Technology                                                  |
| --------------- | ----------------------------------------------------------- |
| CLI framework   | `cobra`                                                     |
| HTTP client     | `net/http` with connection pooling                          |
| YAML parsing    | `gopkg.in/yaml.v3`                                          |
| Concurrency     | goroutines + `errgroup`                                     |
| PDF generation  | `go-pdf` or `jung-kurt/gofpdf`                              |
| EPUB generation | `go-epub`                                                   |
| MOBI            | Shell out to Calibre `ebook-convert`                        |
| Plugin scrapers | Go plugin interface; Python scrapers called as subprocesses |

### Option B — Python CLI (Faster to prototype)

| Layer           | Technology           |
| --------------- | -------------------- |
| CLI framework   | `typer` or `click`   |
| HTTP client     | `httpx` (async)      |
| HTML parsing    | `BeautifulSoup4`     |
| YAML parsing    | `pyyaml`             |
| PDF             | `img2pdf`            |
| EPUB            | `ebooklib`           |
| MOBI            | Shell out to Calibre |
| Package manager | `uv`                 |

> **Recommendation:** Go for the CLI binary (distribution story, single binary, concurrency); Python for custom scraper plugins (richer ecosystem, faster to write). This mirrors the existing Go + Python hybrid architecture pattern used in the broader platform.

---

## 9. Out of Scope (v1)

- GUI or web interface
- Cloud sync or remote library management
- DRM-protected or login-required sources
- Automatic source discovery or AI-generated scrapers
- Account-based authentication (cookie injection) — **v2 candidate**
- Community source registry — **v2 candidate**
- Reading interface / manga viewer

---

## 10. Success Metrics

| Metric                    | Target                                                                       |
| ------------------------- | ---------------------------------------------------------------------------- |
| New source via YAML only  | Configurable in under 5 minutes with no code changes                         |
| Bulk download reliability | A 100-chapter manga completes without manual intervention                    |
| Format compatibility      | Output PDF/EPUB opens correctly on Kindle, Apple Books, and a desktop reader |
| Resume fidelity           | Resuming an interrupted download skips 100% of already-completed images      |

---

## 11. Open Questions

| #   | Question                                                                                             | Impact                |
| --- | ---------------------------------------------------------------------------------------------------- | --------------------- |
| 1   | Should custom scraper plugins be sandboxed (security risk from user-provided code)?                  | Security model        |
| 2   | Should a community `sources.yml` registry exist (like a Helm chart repo) for sharing source configs? | Distribution strategy |
| 3   | Should volume/tankobon grouping be derived from chapter metadata or user-defined mapping?            | Merge UX              |
| 4   | Should login support (cookie injection) be in v1 for sources behind soft paywalls?                   | Scope                 |
| 5   | Should the tool support watching a manga for new chapters and auto-downloading?                      | Feature roadmap       |