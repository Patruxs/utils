---
name: github-release-build
description: Build, verify, and publish complete UTILS GitHub Release assets. Use when Codex needs to create, update, validate, or debug a GitHub Release, release tag, release asset set, install.sh/install.ps1 compatibility, cross-platform binary packaging, or README installer readiness for Windows, Linux, and macOS.
---

# GitHub Release Build

Use this skill for UTILS release work. A release is ready only when GitHub contains Windows, Linux, and macOS assets for both `amd64` and `arm64`.

## Required Assets

For tag `vX.Y.Z`, the latest GitHub Release must contain:

```text
utils_vX.Y.Z_windows_amd64.zip
utils_vX.Y.Z_windows_arm64.zip
utils_vX.Y.Z_linux_amd64.tar.gz
utils_vX.Y.Z_linux_arm64.tar.gz
utils_vX.Y.Z_darwin_amd64.tar.gz
utils_vX.Y.Z_darwin_arm64.tar.gz
```

Windows archives must contain `utils.exe` at archive root. Linux/macOS archives must contain `utils` at archive root. `install.ps1` and `install.sh` depend on those names.

## Workflow

1. Check the worktree and preserve unrelated changes:

```powershell
git status --short --branch
```

2. Test with a repo-local Go cache:

```powershell
$env:GOCACHE='C:\Data\UTILS\.gocache'; go test ./...
```

3. Build and locally verify all six assets:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\.codex\skills\github-release-build\scripts\build-release-assets.ps1 -Version vX.Y.Z
```

4. Create or update the GitHub Release only after local asset verification passes:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\.codex\skills\github-release-build\scripts\build-release-assets.ps1 -Version vX.Y.Z -Repo Patruxs/utils -Upload -CreateRelease
```

5. Verify uploaded assets:

```powershell
gh release view vX.Y.Z -R Patruxs/utils --json assets --jq '.assets[].name'
```

6. Claim README installer readiness only when the latest release is the published tag and contains all six assets.

## Agent Rules

- Do not publish a partial release unless the user explicitly asks for a platform-limited release.
- Do not say Linux/macOS installers work until the `.tar.gz` assets exist on GitHub.
- Do not say Windows installers work until both Windows `.zip` assets exist on GitHub.
- If `go test ./...` fails because the default Go cache is inaccessible on Windows, rerun with `$env:GOCACHE='C:\Data\UTILS\.gocache'`.
- If `gh` is unavailable, build and verify assets locally, then report that upload needs GitHub CLI or another approved upload path.
- Never require administrator/root elevation for release build, install, or verification commands.
