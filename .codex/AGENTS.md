# UTILS Codex Guide

UTILS is a local developer utility hub built with Go 1.26+, Bubble Tea, Bubbles, and Lip Gloss. Treat it as a safety-sensitive terminal utility: default to dry-run, avoid privilege escalation, keep destructive actions explicit and cancellable, and preserve unrelated user changes.

## Architecture

- Follow Bubble Tea MVU: `Init`, `Update`, `View`; never mutate model state in `View()`.
- Run side effects only through `tea.Cmd` values returned from `Update()`.
- Use one `ViewState` enum for complex UI transitions; do not add mode booleans.
- Keep `internal/core/` independent from terminal rendering; core must not import `internal/ui`.
- UI may call core only through commands or command-producing helpers.
- New main features implement `AppFeature` with `Title()`, `Description()`, and `Model()`.
- Wire features through `ui.NewRouter()` from `cmd/tui/main.go`; do not hardcode routes.
- Move OS names, command names/args, layout dimensions, colors, file modes, target labels, magic numbers, and shared copy into package-local `constants.go` files.

## Project Map

- Entry point: `cmd/tui/main.go`
- Router and feature registry: `internal/ui/router.go`
- Shared UI primitives: `internal/ui/common/`
- Cleaner UI: `internal/ui/views/cleaner.go`
- Cleaner core: `internal/core/cleaner/cleaner.go`
- Cleaner targets: `internal/core/cleaner/targets.go`
- Black-box tests: `tests/`
- Internal tests: beside code only when unexported behavior needs direct coverage

## Core And DI

- Prefer injected interfaces over direct OS APIs.
- The cleaner exposes `FileSystem`, `CommandRunner`, and `Cleaner`.
- Use `golang.org/x/sync/errgroup` and goroutines for I/O-bound inspections, scans, and deletions.
- Protect concurrently updated reports/state with `sync.Mutex` or an equivalent safe design.
- Use `log/slog` for structured text logs.
- Keep target data in `targets.go`; keep execution behavior in `cleaner.go`.

## Safety

- Never request, assume, or require administrator/root elevation.
- Do not write scripts, workflows, commands, or utilities that require elevation.
- Destructive actions default to dry-run and execute only after explicit user confirmation.
- Tests must never touch real credentials, shell history, browser profiles, user profile state, system credential stores, or cloud resources.
- Cleaner paths must stay under the current user profile unless a reviewed safety change says otherwise.
- Preserve symlink and outside-home protections.
- SSH key cleanup is opt-in through `Options.CleanSSHKeys`.
- Browser profile cleanup is opt-in through `Options.IncludeBrowserProfiles`.
- Windows Credential Manager cleanup is opt-in and allowlist-based.

## Infrastructure And Deployment

- Application Deployment Helper workflows are container-first.
- Deployment outputs are Docker images pushed to Docker Hub.
- Do not add legacy JAR build or execution paths unless the project direction changes explicitly.
- Infrastructure & Cloud Manager workflows prioritize safe stop/start routines.
- Do not use destructive routine maintenance commands such as `terraform destroy`.

## GitHub Releases

- Before creating, updating, validating, or debugging a release, use `.codex/skills/github-release-build/SKILL.md`.
- A complete release has exactly these six assets:
  - `utils_<tag>_windows_amd64.zip`
  - `utils_<tag>_windows_arm64.zip`
  - `utils_<tag>_linux_amd64.tar.gz`
  - `utils_<tag>_linux_arm64.tar.gz`
  - `utils_<tag>_darwin_amd64.tar.gz`
  - `utils_<tag>_darwin_arm64.tar.gz`
- Do not claim README installers are ready until the latest release contains all six assets.
- On Windows, use the repo-local Go cache for tests and cross-builds:

```powershell
$env:GOCACHE='C:\Data\UTILS\.gocache'
```

- Release archives must place `utils` or `utils.exe` at archive root for `install.sh` and `install.ps1`.

## Coding Standards

- Follow existing package boundaries, naming, and local helpers.
- Use `filepath.Join` for local filesystem paths.
- Search with `rg`; edit manually with `apply_patch`.
- Run `gofmt` on edited Go files.
- Do not remove cleanup targets or business logic unless asked.
- Preserve unrelated dirty-worktree changes.

## Testing

- Prefer black-box tests under `tests/`.
- Use `t.TempDir()`, `t.Setenv()`, `testdata/`, fake inputs, `MockFileSystem`, and `MockCommandRunner`.
- Drive Bubble Tea model tests with `tea.Msg` values.
- Add tests with feature or behavior changes.

Run the full suite with a repo-local Go cache on Windows:

```powershell
$env:GOCACHE='C:\Data\UTILS\.gocache'; go test ./...
```

Normal command:

```powershell
go test ./...
```

## Current Notes

- Cleaner reports are thread-safe while target cleanup runs concurrently.
- Logs use `log/slog` text handling.
- Running cleanup can be canceled from the UI with `esc` or `ctrl+c`.
- Ignore stray untracked workspace temp file `internal/ui/views/cleaner.go.2901840332506010148` unless asked to clean debris.
