# UTILS

A single-binary terminal app for developer machines. Two tools, one screen each:

- **Cleaner** removes local credentials and tokens. It shows exactly what it would delete before you confirm, and it never deletes outside your user profile.
- **Network** shows your current adapter, IP, DNS, and MTU, and lets you switch DNS, flush caches, edit the hosts file, and clear browser caches. Every change is listed before it runs.

Works on Linux, macOS, and Windows (`amd64` and `arm64`). No Go, Node, Python, or admin rights needed to install.

## Install

Linux and macOS:

```sh
curl --proto '=https' -fsSL https://raw.githubusercontent.com/Patruxs/utils/main/install.sh | bash
```

Or with Homebrew:

```sh
brew tap Patruxs/tap && brew install utils
```

Windows (PowerShell):

```powershell
$i = Join-Path $env:TEMP 'utils-install.ps1'
Invoke-WebRequest 'https://raw.githubusercontent.com/Patruxs/utils/main/install.ps1' -OutFile $i
powershell -NoProfile -ExecutionPolicy Bypass -File $i -AddToPath
```

Or with Scoop:

```powershell
scoop bucket add utils https://github.com/Patruxs/scoop-bucket.git
scoop install utils
```

The installers verify the download against the release `checksums.txt` and refuse to install on mismatch. Default install directory is `~/.local/bin` on Linux and macOS, `%USERPROFILE%\utils_bin` on Windows. Set `UTILS_INSTALL_DIR` to change it.

## Use

```sh
utils
```

| Key | Does |
| --- | --- |
| `tab`, `1`, `2` | Switch between Cleaner and Network |
| `↑` `↓` | Move |
| `space` | Toggle an option, or add a network action to a batch |
| `enter` | Run (asks for confirmation before anything is deleted or changed) |
| `d` | Cleaner: save the current preview as a dry-run log |
| `r` | Network: refresh the status card |
| `esc` | Cancel a running action, or go back |
| `?` | Help |
| `q` | Quit |

Network changes need `sudo` on Linux and macOS and run `sudo -v` first so it is cached. On Windows they ask for elevation.

Other commands:

```sh
utils --version
utils --update
utils --uninstall
utils --showPath
```

If you installed with Homebrew or Scoop, use `brew upgrade utils` or `scoop update utils` instead of `--update`.

## Cleaner scope

Always on: credential and token files for cloud CLIs, Git, Docker, Kubernetes, package managers, and AI tools.

Opt-in: SSH keys, shell and tool history, browser profiles, Windows Credential Manager entries, force-stopping running browsers and editors, and a full tool reset that removes whole tool folders and settings.

The preview lists every file before you confirm. Deletion is not undoable, so revoke remote tokens and sessions from their admin portals afterwards.

## Develop

```sh
go run ./cmd/tui
go test ./...
go build -o bin/utils ./cmd/tui
```

Releases are published by tagging `vX.Y.Z`. See [docs/RELEASE.md](docs/RELEASE.md).
