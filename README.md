# UTILS Developer Hub

UTILS is a small terminal utility hub for scripts and workflows used during local development, network diagnostics, configuration, and cleanup tasks.

The project is written in Go and builds into a single executable. After it is built, you do not need Node.js, Python, or any other scripting runtime to run the utility.

## Current Utilities

| Area | Feature | What it does |
| --- | --- | --- |
| Cleaner | System & Credential Cleaner | Dry-run-first local cleanup for developer machines. |
| Cleaner | Dry-run mode | Shows what would be deleted without removing files. |
| Cleaner | Execute mode | Deletes selected matching local files. |
| Cleaner | Developer credentials/configs | Cleans local cloud, Git, Docker, Kubernetes, package-manager, and IaC credentials/configs. |
| Cleaner | AI tool data | Cleans Codex, Gemini, Antigravity, and Claude local data. |
| Cleaner | IDE data | Cleans Visual Studio, VS Code, VS Code Insiders, and VSCodium auth/cache/data/history. |
| Cleaner | Copilot data | Cleans GitHub Copilot auth, config, cache, and extension data. |
| Cleaner | SSH cleanup | Optionally includes SSH config, known hosts, and key files. |
| Cleaner | Shell/tool history | Cleans shell, REPL, database, debugger, and CLI history files. |
| Cleaner | Browser cache cleanup | Cleans Chrome/Chromium, Edge, Brave, CocCoc, Firefox, and Safari caches where supported. |
| Cleaner | Browser profile cleanup | Optionally removes browser sign-ins, cookies, sessions, passwords, extensions, storage, history, and bookmarks. |
| Cleaner | Windows Credential Manager | Optionally deletes allowlisted developer credentials on Windows. |
| Cleaner | Force-stop target apps | Optionally stops browsers, IDEs, and AI apps before cleanup. |
| Cleaner | Cleanup log | Writes a structured cleanup log under the current user profile. |
| Cleaner | User-profile safety guard | Refuses to delete paths outside the current user profile. |
| Network | Network & Diagnostics Manager | Inspects, diagnoses, cleans caches, and configures networking. |
| Network | View current config | Shows adapter, DNS, IP, MTU, DoH, hosts, and ping information. |
| Network | Diagnostics | Tests connectivity, DNS resolution, and ping quality. |
| Network | Apply network config | Applies DNS, DoH where supported, and MTU 1500. |
| Network | Cloudflare DNS | Sets `1.1.1.1` and `1.0.0.1`. |
| Network | Google DNS | Sets `8.8.8.8` and `8.8.4.4`. |
| Network | OpenDNS | Sets `208.67.222.222` and `208.67.220.220`. |
| Network | Quad9 DNS | Sets `9.9.9.9` and `149.112.112.112`. |
| Network | Flush DNS cache | Flushes OS DNS caches. |
| Network | Enable DoH | Enables Windows DNS over HTTPS templates where supported. |
| Network | Disable DoH | Removes Windows DNS over HTTPS entries where supported. |
| Network | Optimize network settings | Applies TCP/MTU optimizations. |
| Network | Reset network optimizations | Resets TCP/Winsock or best-effort platform equivalents. |
| Network | Reset DNS | Restores automatic/default resolver behavior. |
| Network | Reset defaults | Resets DNS, disables DoH where supported, and clears persistent DNS settings. |
| Network | Hosts view | Reads the hosts file. |
| Network | Hosts add | Adds an IP/domain hosts entry. |
| Network | Hosts remove custom | Removes custom hosts entries while preserving defaults/comments. |
| Network | Hosts backup | Creates `hosts.backup`. |
| Network | Hosts restore | Restores from `hosts.backup`. |
| Network | Clear Chrome/Chromium cache | Clears Chrome and Chromium cache/code-cache paths. |
| Network | Clear Firefox cache | Clears Firefox `cache2` folders. |
| Network | Clear Edge cache | Clears Microsoft Edge cache/code-cache paths. |
| Network | Clear Brave cache | Clears Brave cache/code-cache paths. |
| Network | Clear Opera cache | Clears Opera cache/code-cache paths. |
| Network | Clear all browser caches | Runs all supported browser cache cleaners. |
| Network | Persistent DNS status | Shows saved persistent DNS mode and values. |
| Network | Persistent DNS toggle | Turns persistent DNS mode on or off. |
| Network | Persistent DNS apply | Applies saved DNS values. |
| Network | Persistent DNS clear | Removes saved persistent DNS settings. |

## Requirements

To install and run a published UTILS binary, you do not need Go, GitHub CLI, Node.js, Python, or administrator permissions.

To run from source or build the executable, install Go matching the version in `go.mod`.

## Installation

### Linux & macOS

Install with the shell installer:

```sh
curl -fsSL https://raw.githubusercontent.com/Patruxs/utils/main/install.sh | bash
```

The installer supports the same environment variables as `install.sh`: `UTILS_REPO`, `UTILS_BIN`, and `UTILS_INSTALL_DIR`.

Alternatively, install with Homebrew:

```sh
brew tap Patruxs/tap
brew install utils
```

### Windows

Install with the PowerShell installer:

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

Windows users can also install with Scoop after the first release is published:

```powershell
scoop bucket add utils https://github.com/Patruxs/scoop-bucket.git
scoop install utils
```

If you see `Access to the path 'C:\WINDOWS\System32\install.ps1' is denied`, you are using an old command that saves the installer into the current directory. Use the `$env:TEMP` command above instead.

If the installer says `No published GitHub Release found`, UTILS has not published a downloadable Windows binary yet. The Windows installer does not require Go or GitHub CLI, but it does require a published GitHub Release asset such as `utils_v0.1.0_windows_amd64.zip`.

More release and package-manager details are in [.codex/RELEASE.md](.codex/RELEASE.md).

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

## Most Use Commands

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


