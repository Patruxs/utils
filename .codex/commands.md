# Commands

Run from source:

```powershell
go run ./cmd/tui
```

Test:

```powershell
go test ./...
$env:GOCACHE='C:\Data\UTILS\.gocache'; go test ./...
```

Build:

```powershell
go build ./cmd/tui
go build -o bin/utils.exe ./cmd/tui
```

Search:

```powershell
rg --files
rg "pattern"
```

Worktree and format:

```powershell
git status --short
gofmt -w <files>
```

Release assets:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\.codex\skills\github-release-build\scripts\build-release-assets.ps1 -Version v0.1.1
```

Create or update a complete GitHub Release:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\.codex\skills\github-release-build\scripts\build-release-assets.ps1 -Version v0.1.1 -Repo Patruxs/utils -Upload -CreateRelease
```
