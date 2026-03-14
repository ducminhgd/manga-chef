# CI Setup Guide

This document covers the one-time repository configuration required after the
GitHub Actions workflows are pushed.

---

## Branch Protection Rules

Go to **Settings → Branches → Add branch protection rule** for `main`.

Enable the following:

| Setting | Value |
|---|---|
| Branch name pattern | `main` |
| Require a pull request before merging | ✅ |
| Required approvals | `1` |
| Require status checks to pass before merging | ✅ |
| Required status checks | `Format Check`, `Lint`, `Test (race detector)` |
| Require branches to be up to date before merging | ✅ |
| Do not allow bypassing the above settings | ✅ |
| Allow force pushes | ❌ |
| Allow deletions | ❌ |

The required status check names must match the `name:` fields in `ci.yml` exactly:
- `Format Check`
- `Lint`
- `Test (race detector)`

---

## Required Repository Secrets

No secrets are required for the CI pipeline to function. The release job uses
`GITHUB_TOKEN`, which is automatically provided by GitHub Actions.

If you later integrate SonarQube or a coverage reporting service (e.g. Codecov),
add the relevant token under **Settings → Secrets and variables → Actions**.

---

## Workflow Summary

| Workflow | File | Trigger | Purpose |
|---|---|---|---|
| **CI** | `ci.yml` | push/PR to `main` | Quality gate: fmt, lint, test with race detector, coverage report |
| **Build** | `build.yml` | push/PR to `main`, tags `v*` | Compile binaries for all platforms; publish release assets on tags |

### CI workflow job graph

```
push / pull_request
        │
        ▼
   ┌─────────┐
   │   fmt   │  (fast fail ~5s)
   └────┬────┘
        │
   ┌────┴─────────────┐
   │                  │
   ▼                  ▼
┌──────┐        ┌──────────────────┐
│ lint │        │ test (race det.) │
└──────┘        └──────────────────┘
                        │
                        ▼
                  coverage.out
                  → job summary
                  → artifact (7d)
```

### Build workflow matrix

| Platform | Arch | Runner | CGO |
|---|---|---|---|
| linux | amd64 | ubuntu-latest | disabled |
| linux | arm64 | ubuntu-latest (cross) | disabled |
| darwin | amd64 | macos-latest | disabled |
| darwin | arm64 | macos-latest (cross) | disabled |
| windows | amd64 | windows-latest | disabled |

All binaries are statically linked (`CGO_ENABLED=0`) for maximum portability.

### Creating a release

```bash
git tag v1.0.0
git push origin v1.0.0
```

The build workflow detects the `v*` tag, builds all platform binaries,
generates a `SHA256SUMS.txt`, and creates a GitHub Release with all artifacts
attached. Release notes are auto-generated from merged PRs.

---

## Local Pre-push Gate

Mirrors exactly what CI will check. Run before every push:

```bash
make check
```

This runs: `fmt-check` → `lint` → `test-race`