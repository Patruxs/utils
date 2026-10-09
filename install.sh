#!/usr/bin/env bash
set -euo pipefail

repo="${UTILS_REPO:-Patruxs/utils}"
bin_name="${UTILS_BIN:-utils}"
install_dir="${UTILS_INSTALL_DIR:-"$HOME/.local/bin"}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: '$1' is required but was not found on PATH" >&2
    exit 1
  fi
}

need curl
need awk
need find
need grep
need mktemp
need sed
need tar
need uname

if command -v sha256sum >/dev/null 2>&1; then
  sha256_of() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  sha256_of() { shasum -a 256 "$1" | awk '{print $1}'; }
else
  echo "error: 'sha256sum' or 'shasum' is required to verify the download" >&2
  exit 1
fi

case "$(uname -s)" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *)
    echo "error: unsupported OS '$(uname -s)'; use the Windows PowerShell installer on Windows" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *)
    echo "error: unsupported architecture '$(uname -m)'" >&2
    exit 1
    ;;
esac

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

release_json="$tmp_dir/release.json"
archive="$tmp_dir/${bin_name}.tar.gz"
checksums="$tmp_dir/checksums.txt"

echo "Fetching latest UTILS release metadata..."
curl --proto '=https' -fsSL "https://api.github.com/repos/${repo}/releases/latest" -o "$release_json"

download_urls="$(
  grep -Eo '"browser_download_url"[[:space:]]*:[[:space:]]*"[^"]+"' "$release_json" |
    sed -E 's/.*"([^"]+)"/\1/' || true
)"
asset_url="$(printf '%s\n' "$download_urls" | grep -E "/${bin_name}_.+_${os}_${arch}\.tar\.gz$" | head -n 1 || true)"
checksums_url="$(printf '%s\n' "$download_urls" | grep -E '/checksums\.txt$' | head -n 1 || true)"

if [ -z "$asset_url" ]; then
  echo "error: could not find a ${os}/${arch} tarball in the latest ${repo} release" >&2
  exit 1
fi

if [ -z "$checksums_url" ]; then
  echo "error: the latest ${repo} release has no checksums.txt; refusing to install an unverified binary" >&2
  exit 1
fi

echo "Downloading ${asset_url}..."
curl --proto '=https' -fsSL "$checksums_url" -o "$checksums"
curl --proto '=https' -fsSL "$asset_url" -o "$archive"

asset_name="${asset_url##*/}"
expected_sha256="$(awk -v name="$asset_name" '$2 == name || $2 == "*" name { print tolower($1); exit }' "$checksums")"
if [ -z "$expected_sha256" ]; then
  echo "error: checksums.txt has no entry for ${asset_name}; refusing to install an unverified binary" >&2
  exit 1
fi
actual_sha256="$(sha256_of "$archive")"
if [ "$actual_sha256" != "$expected_sha256" ]; then
  echo "error: checksum mismatch for ${asset_name}; refusing to install" >&2
  exit 1
fi
echo "Verified sha256 of ${asset_name}."

echo "Installing ${bin_name} to ${install_dir}..."
mkdir -p "$install_dir"
tar -xzf "$archive" -C "$tmp_dir"

binary_path="$(find "$tmp_dir" -type f \( -name "$bin_name" -o -name "${bin_name}.exe" \) | head -n 1)"
if [ -z "$binary_path" ]; then
  echo "error: archive did not contain ${bin_name}" >&2
  exit 1
fi

cp "$binary_path" "${install_dir}/${bin_name}"
chmod 0755 "${install_dir}/${bin_name}"

echo "Installed: ${install_dir}/${bin_name}"
if ! printf '%s' ":$PATH:" | grep -Fq ":${install_dir}:"; then
  echo "Add this directory to PATH if needed:"
  echo "  export PATH=\"${install_dir}:\$PATH\""
fi
