# Release And Installation

UTILS releases use GitHub Actions and GoReleaser to build Linux, macOS, and Windows binaries for `amd64` and `arm64`, publish GitHub Release assets, and update Homebrew and Scoop package repositories.

All install paths use standard user permissions only: no `sudo`, admin prompts, or system-wide install directories. One-line installers require a published GitHub Release; if `/releases/latest` returns `404 Not Found`, no public latest release exists yet or the repo is private to unauthenticated users.

## GitHub Setup

- Create `Patruxs/utils`.
- Create `Patruxs/homebrew-tap`.
- Create `Patruxs/scoop-bucket`.
- In `Patruxs/utils`, enable GitHub Actions with write access to repository contents.
- Create a GitHub PAT that can write to `Patruxs/utils`, `Patruxs/homebrew-tap`, and `Patruxs/scoop-bucket`.
- Add the PAT to `Patruxs/utils` as Actions secret `GORELEASER_PAT`.
- Keep the default `GITHUB_TOKEN`; it can publish current-repo releases, but the PAT is required for cross-repo tap and bucket pushes.
- If `Resolve release publishing token` reports a missing `GORELEASER_PAT`, the GitHub Actions workflow skips GoReleaser package publishing. Use the local `github-release-build` script to publish release assets, or add the secret to enable automated GoReleaser publishing.
- If Homebrew/Scoop publishing fails, verify the package repos exist and the PAT can write repository contents.
- If a tag exists but `/releases/latest` returns `404`, that tag workflow did not publish a release. Inspect it with:

```powershell
gh run list -R Patruxs/utils --workflow release.yml --limit 10
gh run view <run-id> -R Patruxs/utils --log-failed
```

Push a version tag to trigger release:

```powershell
git tag v0.1.0
git push origin v0.1.0
```

## Homebrew

After the first successful release:

```sh
brew tap Patruxs/tap
brew install utils
```

## Scoop

After the first successful release:

```powershell
scoop bucket add utils https://github.com/Patruxs/scoop-bucket.git
scoop install utils
```

## Linux/macOS One-Liner

Installs the latest release into `~/.local/bin`:

```sh
curl -fsSL https://raw.githubusercontent.com/Patruxs/utils/main/install.sh | bash
```

The URL can be shortened after the repo is public.

## Windows PowerShell One-Liner

Downloads the latest Windows release, extracts it to `$env:USERPROFILE\utils_bin`, and runs `utils.exe` without admin rights:

```powershell
$repo='Patruxs/utils'; $bin='utils'; $arch=if([Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq 'Arm64'){'arm64'}else{'amd64'}; $dir=Join-Path $env:USERPROFILE 'utils_bin'; New-Item -ItemType Directory -Force $dir | Out-Null; try { $release=Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest" -ErrorAction Stop } catch { throw "No published GitHub Release found for $repo. Run from source with 'go run ./cmd/tui', or publish a release first." }; $asset=$release.assets | Where-Object { $_.name -match "$bin`_.+_windows_$arch\.zip$" } | Select-Object -First 1; if(-not $asset){ throw "No Windows $arch release asset found in the latest GitHub Release" }; $zip=Join-Path $env:TEMP $asset.name; Invoke-WebRequest $asset.browser_download_url -OutFile $zip; Expand-Archive -Path $zip -DestinationPath $dir -Force; & (Join-Path $dir "$bin.exe")
```

Run later:

```powershell
& "$env:USERPROFILE\utils_bin\utils.exe"
```
