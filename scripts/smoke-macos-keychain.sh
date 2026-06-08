#!/usr/bin/env bash
set -euo pipefail

evidence_path="${UTILS_KEYCHAIN_SMOKE_EVIDENCE_PATH:-}"

write_keychain_evidence() {
  local status="$1"
  local message="$2"
  if [[ -z "$evidence_path" ]]; then
    return 0
  fi

  mkdir -p "$(dirname "$evidence_path")"
  cat >"$evidence_path" <<EOF
{
  "kind": "utils-macos-keychain-smoke",
  "platform": "$(uname -s)",
  "status": "$status",
  "timestamp_utc": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "command": "go test -tags keychain_smoke ./tests/core/accounts -run TestMacOSKeychainSmoke -count=1",
  "message": "$message"
}
EOF
}

if [[ "$(uname -s)" != "Darwin" ]]; then
  write_keychain_evidence "fail" "macOS Keychain smoke requires Darwin"
  echo "macOS Keychain smoke must run on macOS." >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
keychain_path="$tmp_dir/utils-smoke.keychain-db"
keychain_password="utils-smoke-$(date +%s)-$$"
old_keychains_file="$tmp_dir/keychains.txt"
old_default_keychain_file="$tmp_dir/default-keychain.txt"

restore_keychains() {
  if [[ -f "$old_default_keychain_file" ]]; then
    old_default_keychain="$(cat "$old_default_keychain_file")"
    if [[ -n "$old_default_keychain" ]]; then
      security default-keychain -d user -s "$old_default_keychain" >/dev/null
    fi
  fi
  if [[ -f "$old_keychains_file" ]]; then
    old_keychains=()
    while IFS= read -r old_keychain; do
      old_keychains+=("$old_keychain")
    done <"$old_keychains_file"
    if [[ "${#old_keychains[@]}" -gt 0 ]]; then
      security list-keychains -d user -s "${old_keychains[@]}" >/dev/null
    fi
  fi
  security delete-keychain "$keychain_path" >/dev/null 2>&1 || true
  rm -rf "$tmp_dir"
}
trap restore_keychains EXIT

security list-keychains -d user | sed 's/^ *"//; s/"$//' >"$old_keychains_file"
security default-keychain -d user | sed 's/^ *"//; s/"$//' >"$old_default_keychain_file"
security create-keychain -p "$keychain_password" "$keychain_path"
security set-keychain-settings -lut 21600 "$keychain_path"
security unlock-keychain -p "$keychain_password" "$keychain_path"
security list-keychains -d user -s "$keychain_path"
security default-keychain -d user -s "$keychain_path"

export UTILS_KEYCHAIN_SMOKE=1
export HOME="$tmp_dir/home"
mkdir -p "$HOME"

cd "$repo_root"
if go test -tags keychain_smoke ./tests/core/accounts -run TestMacOSKeychainSmoke -count=1; then
  write_keychain_evidence "pass" "macOS Keychain smoke passed"
else
  write_keychain_evidence "fail" "macOS Keychain smoke failed"
  exit 1
fi
