param(
  [Parameter(Mandatory = $true)]
  [ValidatePattern('^v\d+\.\d+\.\d+(-[A-Za-z0-9.-]+)?$')]
  [string]$Version,

  [string]$Repo = "Patruxs/utils",
  [string]$Target = "main",
  [string]$DistDir = "",
  [switch]$SkipTests,
  [switch]$Upload,
  [switch]$CreateRelease
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Step {
  param([string]$Message)
  Write-Host "==> $Message"
}

function Require-Command {
  param([string]$Name)
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Required command '$Name' was not found on PATH."
  }
}

function New-TarGz {
  param(
    [string]$SourceDir,
    [string]$ArchivePath
  )
  tar -czf $ArchivePath -C $SourceDir utils
}

function Assert-ReleaseAssets {
  param(
    [string]$Directory,
    [string]$Tag
  )

  $required = @(
    "utils_${Tag}_windows_amd64.zip",
    "utils_${Tag}_windows_arm64.zip",
    "utils_${Tag}_linux_amd64.tar.gz",
    "utils_${Tag}_linux_arm64.tar.gz",
    "utils_${Tag}_darwin_amd64.tar.gz",
    "utils_${Tag}_darwin_arm64.tar.gz"
  )

  foreach ($name in $required) {
    $path = Join-Path $Directory $name
    if (-not (Test-Path -LiteralPath $path)) {
      throw "Missing release asset: $name"
    }
    if ((Get-Item -LiteralPath $path).Length -le 0) {
      throw "Release asset is empty: $name"
    }
  }

  return $required
}

Require-Command go
Require-Command tar

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..\..\..")).Path
if (-not $DistDir) {
  $DistDir = Join-Path $repoRoot "dist\release-$Version"
}

New-Item -ItemType Directory -Force -Path $DistDir | Out-Null

$env:GOCACHE = Join-Path $repoRoot ".gocache"

if (-not $SkipTests) {
  Write-Step "Running tests"
  Push-Location $repoRoot
  try {
    go test ./...
  } finally {
    Pop-Location
  }
}

$targets = @(
  @{ GOOS = "windows"; GOARCH = "amd64"; Ext = ".exe"; Archive = "zip" },
  @{ GOOS = "windows"; GOARCH = "arm64"; Ext = ".exe"; Archive = "zip" },
  @{ GOOS = "linux"; GOARCH = "amd64"; Ext = ""; Archive = "tar.gz" },
  @{ GOOS = "linux"; GOARCH = "arm64"; Ext = ""; Archive = "tar.gz" },
  @{ GOOS = "darwin"; GOARCH = "amd64"; Ext = ""; Archive = "tar.gz" },
  @{ GOOS = "darwin"; GOARCH = "arm64"; Ext = ""; Archive = "tar.gz" }
)

Push-Location $repoRoot
try {
  foreach ($targetItem in $targets) {
    $name = "utils_${Version}_$($targetItem.GOOS)_$($targetItem.GOARCH)"
    $buildDir = Join-Path $DistDir $name
    New-Item -ItemType Directory -Force -Path $buildDir | Out-Null

    $env:GOOS = $targetItem.GOOS
    $env:GOARCH = $targetItem.GOARCH
    $env:CGO_ENABLED = "0"

    $binaryName = "utils$($targetItem.Ext)"
    Write-Step "Building $name"
    go build -ldflags "-s -w -X main.version=$Version" -o (Join-Path $buildDir $binaryName) ./cmd/tui

    if ($targetItem.Archive -eq "zip") {
      $archive = Join-Path $DistDir "$name.zip"
      Compress-Archive -Path (Join-Path $buildDir "*") -DestinationPath $archive -Force
    } else {
      $archive = Join-Path $DistDir "$name.tar.gz"
      New-TarGz -SourceDir $buildDir -ArchivePath $archive
    }
  }
} finally {
  Remove-Item Env:GOOS -ErrorAction SilentlyContinue
  Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
  Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
  Pop-Location
}

$assets = Assert-ReleaseAssets -Directory $DistDir -Tag $Version
Write-Step "Built and verified all required assets"
$assets | ForEach-Object { Write-Host "  $_" }

if ($Upload) {
  Require-Command gh

  $releaseExists = $false
  try {
    gh release view $Version -R $Repo *> $null
    $releaseExists = $true
  } catch {
    $releaseExists = $false
  }

  if (-not $releaseExists) {
    if (-not $CreateRelease) {
      throw "Release $Version does not exist. Re-run with -CreateRelease or create it first."
    }

    Write-Step "Creating GitHub Release $Version"
    $assetPaths = $assets | ForEach-Object { Join-Path $DistDir $_ }
    gh release create $Version @assetPaths -R $Repo --target $Target --title $Version --notes "Complete UTILS release assets for Windows, Linux, and macOS."
  } else {
    Write-Step "Uploading assets to GitHub Release $Version"
    $assetPaths = $assets | ForEach-Object { Join-Path $DistDir $_ }
    gh release upload $Version @assetPaths -R $Repo --clobber
  }

  Write-Step "Verifying uploaded GitHub Release assets"
  $remoteAssets = gh release view $Version -R $Repo --json assets --jq ".assets[].name"
  foreach ($asset in $assets) {
    if ($remoteAssets -notcontains $asset) {
      throw "Uploaded release is missing asset: $asset"
    }
  }
  Write-Step "GitHub Release $Version contains all required assets"
}
