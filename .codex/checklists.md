# Codex Checklists

## Before Editing

- Read the relevant package.
- Check `git status --short` and preserve unrelated changes.
- Search with `rg` before assuming a symbol or pattern is unused.
- Place the change in the right boundary: core, UI, shared UI, router, or tests.

## Cleaner Targets

- Add path data in `internal/core/cleaner/targets.go`.
- Use `filepath.Join(home, ...)` for home-relative paths.
- Use `envTarget(fs, ...)` for Windows environment-rooted paths.
- Reuse label constants from `internal/core/cleaner/constants.go`.
- Do not remove targets or weaken outside-home checks.
- Run cleaner tests after target logic changes.

## Cleaner Core

- Keep OS calls behind `FileSystem` or `CommandRunner` when practical.
- Preserve public `Run(ctx, opts)`.
- Keep `Report.add` concurrency-safe.
- Preserve dry-run and context cancellation behavior.
- Write logs through the existing structured logging path.

## Cleaner UI

- Drive behavior from `ViewState`; do not add mode booleans.
- Mutate model state only in `Update`.
- Keep `View` side-effect-light and derived from model fields.
- Use `common.Layout` for wrapping/dimensions and `common.DefaultKeys` for global key meanings.
- Add model tests for new user flows.

## Router

- Add features through `AppFeature`.
- Keep `NewRouter(features ...AppFeature)` injectable for tests.
- Keep `DefaultFeatures()` as production wiring.
- Keep placeholder features as simple Bubble Tea models until real models exist.

## Infrastructure And Deployment

- Keep infra routines non-destructive and focused on safe stop/start workflows.
- Do not add routine `terraform destroy` flows.
- Keep deployment container-first: build Docker images and push to Docker Hub.
- Do not add legacy JAR build or execution paths unless explicitly requested.
- Do not require administrator/root elevation.

## GitHub Releases

- Use `.codex/skills/github-release-build/SKILL.md` before release build, upload, or validation.
- Run tests with repo-local cache:

```powershell
$env:GOCACHE='C:\Data\UTILS\.gocache'; go test ./...
```

- Build all six assets:
  - Windows `amd64`, `arm64`: `.zip`
  - Linux `amd64`, `arm64`: `.tar.gz`
  - macOS `amd64`, `arm64`: `.tar.gz`
- Verify asset names match installer patterns before upload.
- After upload, verify the GitHub Release contains all six assets.
- On Windows, smoke-test the Windows installer after publishing.

## Verification

Format edited Go files:

```powershell
gofmt -w <changed-go-files>
```

Run tests:

```powershell
$env:GOCACHE='C:\Data\UTILS\.gocache'; go test ./...
```

Build smoke test:

```powershell
go build ./cmd/tui
```

Use the build smoke test for package wiring, import, or entry-point changes.
