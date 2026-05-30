# UTILS Developer Hub

UTILS is a small terminal utility hub for scripts and workflows used during local development, infrastructure work, deployment preparation, and cleanup tasks.

The project is written in Go and builds into a single executable. After it is built, you do not need Node.js, Python, or any other scripting runtime to run the utility.

## Current Utilities

- System & Credential Cleaner: dry-run-first cleanup for local developer credentials, shell history, IDE authentication/cache/data, browser caches, and optional browser profiles.
- Infrastructure & Cloud Manager: route is prepared for safe start and stop workflows.
- Application Deployment Helper: route is prepared for Docker image build and Docker Hub push preparation.

## Requirements

To install and run a published UTILS binary, you do not need Go, GitHub CLI, Node.js, Python, or administrator permissions.

To run from source or build the executable, install Go matching the version in `go.mod`.

## Install On Windows

This installs UTILS into your user profile and adds it to your user PATH. You can run these commands from any PowerShell directory, including `C:\WINDOWS\System32`.

Open PowerShell and run:

```powershell
$installer = Join-Path $env:TEMP 'utils-install.ps1'
Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/Patruxs/utils/main/install.ps1' -OutFile $installer
powershell -NoProfile -ExecutionPolicy Bypass -File $installer -AddToPath
```

Then open a new PowerShell window and run:

```powershell
utils
```

If you do not use `-AddToPath`, run it from the default install directory:

```powershell
& "$env:USERPROFILE\utils_bin\utils.exe"
```

If you see `Access to the path 'C:\WINDOWS\System32\install.ps1' is denied`, you are using an old command that saves the installer into the current directory. Use the `$env:TEMP` command above instead.

If the installer says `No published GitHub Release found`, UTILS has not published a downloadable Windows binary yet. The Windows installer does not require Go or GitHub CLI, but it does require a published GitHub Release asset such as `utils_v0.1.0_windows_amd64.zip`.

## Install On Linux Or macOS

```sh
curl -fsSL https://raw.githubusercontent.com/Patruxs/utils/main/install.sh | bash
```

The installer supports the same environment variables as `install.sh`: `UTILS_REPO`, `UTILS_BIN`, and `UTILS_INSTALL_DIR`.

Scoop, after the first release is published:

```powershell
scoop bucket add utils https://github.com/Patruxs/scoop-bucket.git
scoop install utils
```

More release and package-manager details are in [RELEASE.md](RELEASE.md).

## Development

These commands are only for contributors who want to run or build UTILS from source.

Run from source:

```powershell
go run ./cmd/tui
```

Windows:

```powershell
go build -o bin/utils.exe ./cmd/tui
```

Linux or macOS:

```sh
go build -o bin/utils ./cmd/tui
```

## Run The Built App

Windows:

```powershell
.\bin\utils.exe
```

Linux or macOS:

```sh
./bin/utils
```

## Self-Management Commands

| Task | Command |
| --- | --- |
| Show the installed executable path | `utils --showPath` |
| Update UTILS to the latest GitHub Release | `utils --update` |
| Remove the currently running UTILS executable | `utils --uninstall` |
| Show the current UTILS version | `utils --version` |
| Show available UTILS commands | `utils --help` |

## Build For Another OS

From PowerShell, set the target OS and architecture before building:

```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o bin/utils-linux-amd64 ./cmd/tui
```

Common `GOOS` values are `windows`, `linux`, and `darwin`. Common `GOARCH` values are `amd64` and `arm64`.

## Test

```powershell
go test ./...
```
