param(
  [string]$Repo = $(if ($env:UTILS_REPO) { $env:UTILS_REPO } else { "Patruxs/utils" }),
  [string]$BinName = $(if ($env:UTILS_BIN) { $env:UTILS_BIN } else { "utils" }),
  [string]$InstallDir = $(if ($env:UTILS_INSTALL_DIR) { $env:UTILS_INSTALL_DIR } else { Join-Path $env:USERPROFILE "utils_bin" }),
  [switch]$AddToPath
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Step {
  param([string]$Message)
  Write-Host "==> $Message"
}

function Get-UtilsArch {
  $arch = [Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()

  switch ($arch) {
    "x64" { return "amd64" }
    "arm64" { return "arm64" }
    default { throw "Unsupported architecture '$arch'. Supported architectures are amd64 and arm64." }
  }
}

function Invoke-Download {
  param(
    [string]$Uri,
    [string]$OutFile
  )

  $params = @{
    Uri = $Uri
    OutFile = $OutFile
  }

  if ($PSVersionTable.PSVersion.Major -lt 6) {
    $params.UseBasicParsing = $true
  }

  Invoke-WebRequest @params
}

function Add-InstallDirToUserPath {
  param([string]$PathToAdd)

  $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
  $pathParts = @($currentPath -split ";" | Where-Object { $_.Trim() -ne "" })
  $normalizedPath = $PathToAdd.TrimEnd("\")
  $alreadyExists = $pathParts | Where-Object { $_.TrimEnd("\") -ieq $normalizedPath } | Select-Object -First 1

  if ($alreadyExists) {
    Write-Step "Install directory is already on the user PATH."
    return
  }

  $newPath = if ($currentPath) { "$currentPath;$PathToAdd" } else { $PathToAdd }
  [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
  $env:Path = "$PathToAdd;$env:Path"
  Write-Step "Added install directory to the user PATH. Open a new terminal to use it everywhere."
}

$arch = Get-UtilsArch
$escapedBin = [regex]::Escape($BinName)
$assetPattern = "^${escapedBin}_.+_windows_${arch}\.zip$"
$tmpDir = Join-Path ([IO.Path]::GetTempPath()) ("utils-install-" + [Guid]::NewGuid().ToString("N"))

try {
  New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
  $archive = Join-Path $tmpDir "${BinName}.zip"
  $extractDir = Join-Path $tmpDir "extract"

  Write-Step "Fetching latest UTILS release metadata..."
  try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
  } catch {
    throw "No published GitHub Release found for $Repo. This installer does not require Go or GitHub CLI, but it does require a published Windows release asset."
  }

  $asset = $release.assets |
    Where-Object { $_.name -match $assetPattern } |
    Select-Object -First 1

  if (-not $asset) {
    throw "Could not find a Windows $arch zip asset in the latest $Repo release."
  }

  Write-Step "Downloading $($asset.browser_download_url)..."
  Invoke-Download -Uri $asset.browser_download_url -OutFile $archive

  Write-Step "Installing $BinName to $InstallDir..."
  New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
  New-Item -ItemType Directory -Force -Path $extractDir | Out-Null
  Expand-Archive -Path $archive -DestinationPath $extractDir -Force

  $binary = Get-ChildItem -LiteralPath $extractDir -Recurse -File |
    Where-Object { $_.Name -ieq "$BinName.exe" -or $_.Name -ieq $BinName } |
    Select-Object -First 1

  if (-not $binary) {
    throw "Archive did not contain $BinName.exe."
  }

  $target = Join-Path $InstallDir "$BinName.exe"
  Copy-Item -LiteralPath $binary.FullName -Destination $target -Force
  Unblock-File -LiteralPath $target -ErrorAction SilentlyContinue

  Write-Step "Installed: $target"

  if ($AddToPath) {
    Add-InstallDirToUserPath -PathToAdd $InstallDir
  } else {
    $pathParts = @($env:Path -split ";" | Where-Object { $_.Trim() -ne "" })
    $normalizedInstallDir = $InstallDir.TrimEnd("\")
    $onPath = $pathParts | Where-Object { $_.TrimEnd("\") -ieq $normalizedInstallDir } | Select-Object -First 1

    if (-not $onPath) {
      Write-Host ""
      Write-Host "Run with -AddToPath to add this directory to your user PATH:"
      Write-Host "  powershell -NoProfile -ExecutionPolicy Bypass -File <path-to-install.ps1> -AddToPath"
    }
  }

  Write-Host ""
  Write-Host "Run UTILS:"
  Write-Host "  & `"$target`""
} finally {
  if (Test-Path -LiteralPath $tmpDir) {
    Remove-Item -LiteralPath $tmpDir -Recurse -Force
  }
}
