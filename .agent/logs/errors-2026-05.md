## [2026-05-30 20:57] - Git Push Failed Over SSH

- **Type**: Process
- **Severity**: Medium
- **File**: `repository remote configuration`
- **Agent**: Codex
- **Root Cause**: The local Git remote used `git@github.com:Patruxs/utils.git`, but this environment did not have a GitHub SSH public key available for that push path.
- **Error Message**:
  ```text
  git@github.com: Permission denied (publickey).
  fatal: Could not read from remote repository.
  ```
- **Fix Applied**: Switched the remote to HTTPS and used GitHub CLI credential integration for authenticated Git operations.
- **Prevention**: Prefer `gh auth setup-git` plus an HTTPS remote in this workspace unless SSH access has been explicitly verified.
- **Status**: Fixed

---

## [2026-05-30 20:59] - Release Workflow Failed Without GORELEASER_PAT

- **Type**: Process
- **Severity**: Medium
- **File**: `.github/workflows/release.yml:25`
- **Agent**: Codex
- **Root Cause**: Creating the `v0.1.1` GitHub Release also created the tag and triggered the GoReleaser workflow, but the repository did not have the `GORELEASER_PAT` secret required by the workflow's package-publishing guard.
- **Error Message**:
  ```text
  Missing GORELEASER_PAT. Create a fine-grained PAT that can write contents to Patruxs/utils, Patruxs/homebrew-tap, and Patruxs/scoop-bucket, then add it as an Actions secret named GORELEASER_PAT.
  ```
- **Fix Applied**: Updated the workflow to emit a notice and skip GoReleaser package publishing when `GORELEASER_PAT` is missing; the local `github-release-build` script remains the release asset publishing path.
- **Prevention**: Add `GORELEASER_PAT` before relying on tag-triggered GoReleaser publishing, or continue publishing assets through `.codex/skills/github-release-build/scripts/build-release-assets.ps1`.
- **Status**: Fixed

---
