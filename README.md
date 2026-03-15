# manga-chef

Manga Chef is a CLI tool to download manga chapters from configured sources (like TruyenQQ) and convert image chapters into structured outputs.

## 🚀 Quick Start

### 1) Build

```bash
go build -o manga-chef ./cmd/manga-chef
```

### 2) Create sources config

Copy `sources.example.yml` into `sources.yml` and adjust the source definitions.

```bash
cp sources.example.yml sources.yml
```

### 3) Download manga from a source

Use `--source` (source code from config) and `--url` (manga page URL).

```bash
./manga-chef download --source truyenqq --url "https://truyenqqno.com/truyen-tranh/dau-an-rong-thieng-236" --workers 4
```

Add `--force` to re-download chapters even if files exist.

### 4) Quiet mode

Suppress progress output with:

```bash
./manga-chef download --source truyenqq --url "..." --quiet
```

## 📁 Output Layout

By default, output directory is derived from the manga URL slug. For example:

`https://truyenqqno.com/truyen-tranh/dau-an-rong-thieng-236` → `dau-an-rong-thieng-236`

You can override with `--output <dir>`.

Downloaded chapters are saved under:

`<out>/<source>/<chapter>/NNN.ext`

Example (default slug output):

```
dau-an-rong-thieng-236/truyenqq/chap-001/001.jpg
```

## ⚙️ Config Reference

`sources.yml` entries include:
- `name`: user-friendly source name
- `code`: short source id (use with `--source`)
- `base_url`: root site URL
- `scraper`: built-in scraper name (e.g. `truyenqq`, `generic`)
- `rate_limit_ms`: optional delay between chapters
- `retries`: number of download retries
- `timeout_s`: per-request timeout seconds

### Example `sources/truyenqq.yml`

```yaml
- name: TruyenQQ
  code: truyenqq
  base_url: https://truyenqqno.com
  scraper: truyenqq
  rate_limit_ms: 500
  retries: 3
  timeout_s: 30
  headers:
    Referer: https://truyenqqno.com
    User-Agent: MangaChef/1.0
```

## 🧪 Run Tests

```bash
go test ./... -count=1
```

## 🛠️ Developer Notes

- Scrapers are in `internal/scraper/*` and register themselves in init.
- Download engine with retry/resume is in `internal/downloader`.
- If the source page structure changes, update scraper selectors (or use generic scraper with CSS selectors).

## 📌 Next Steps

- Add additional sources in `sources/` directory.
- Add converter subcommands when implemented (`convert` / `merge`).

