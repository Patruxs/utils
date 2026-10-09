# Releasing UTILS

Pushing a `v*` tag runs `.github/workflows/release.yml`. It runs the tests, then GoReleaser builds Linux, macOS, and Windows binaries for `amd64` and `arm64`, publishes the GitHub Release with `checksums.txt`, and updates the Homebrew tap and Scoop bucket. The workflow then attests build provenance for every archive in `checksums.txt`.

## One-time setup

- Package repositories: `Patruxs/homebrew-tap` (users run `brew tap Patruxs/tap`) and `Patruxs/scoop-bucket`.
- Actions secret `GORELEASER_PAT` in `Patruxs/utils`: a fine-grained PAT that can write contents to `Patruxs/utils`, `Patruxs/homebrew-tap`, and `Patruxs/scoop-bucket`. Without it the release workflow fails.

## Publish

```sh
git tag vX.Y.Z
git push origin vX.Y.Z
```

Publish releases only through this workflow. `install.sh`, `install.ps1`, and `utils --update` refuse a release without `checksums.txt` or with an archive that does not match it. `v0.1.1` has no `checksums.txt`, so the checksum-verifying installers must be merged together with publishing the next release: push the next tag right after the merge.

## Verify a download

```sh
sha256sum --ignore-missing -c checksums.txt
gh attestation verify utils_vX.Y.Z_linux_amd64.tar.gz -R Patruxs/utils
```

## Troubleshooting

If a tag exists but `/releases/latest` returns `404`, the tag's workflow did not publish a release:

```sh
gh run list -R Patruxs/utils --workflow release.yml --limit 10
gh run view <run-id> -R Patruxs/utils --log-failed
```

If Homebrew tap or Scoop bucket publishing fails, check that both repositories exist and that `GORELEASER_PAT` can write their contents.
