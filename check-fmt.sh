#!/usr/bin/env bash
# scripts/check-fmt.sh
#
# Verifies that all Go source files are formatted with gofmt.
# Exits 1 and prints the offending files if any are unformatted.
#
# Usage:
#   ./scripts/check-fmt.sh              — check all .go files
#   ./scripts/check-fmt.sh ./internal/  — check a specific directory
#
# Integrate with git pre-commit hook:
#   ln -s ../../scripts/check-fmt.sh .git/hooks/pre-commit
#
# Integrate with CI (GitHub Actions):
#   - name: Check formatting
#     run: ./scripts/check-fmt.sh

set -euo pipefail

# ── Colour output ──────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Colour

# ── Resolve target directory ───────────────────────────────────────────────────
TARGET="${1:-.}"

# ── Verify gofmt is available ─────────────────────────────────────────────────
if ! command -v gofmt &>/dev/null; then
  echo -e "${RED}error:${NC} gofmt not found. Install Go: https://go.dev/dl/"
  exit 1
fi

# ── Find unformatted files ─────────────────────────────────────────────────────
# -l  list files that differ from gofmt output (don't modify them)
# -e  report all parse errors, not just the first
#
# We exclude:
#   - vendor/          third-party code we don't own
#   - mock_*.go        generated mocks (may have non-standard formatting)
#   - *_generated.go   other generated files

UNFORMATTED=$(
  find "$TARGET" -name "*.go" \
    ! -path "*/vendor/*" \
    ! -name "mock_*.go" \
    ! -name "*_generated.go" \
    -print0 \
  | xargs -0 gofmt -l -e 2>&1
)

# ── Report results ─────────────────────────────────────────────────────────────
if [[ -z "$UNFORMATTED" ]]; then
  echo -e "${GREEN}✓${NC} All Go files are properly formatted."
  exit 0
fi

echo -e "${RED}✗ The following files are not formatted with gofmt:${NC}"
echo ""

while IFS= read -r file; do
  echo -e "  ${YELLOW}${file}${NC}"
done <<< "$UNFORMATTED"

echo ""
echo -e "Run the following command to fix them:"
echo -e "  ${GREEN}gofmt -w .${NC}"
echo ""
echo -e "Or use the Makefile target:"
echo -e "  ${GREEN}make fmt${NC}"

exit 1