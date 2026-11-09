#!/usr/bin/env bash
set -euo pipefail
: "${KEYSTORE_B64:?Keystore rilis belum dikonfigurasi}"
: "${KEYSTORE_PASSWORD:?Kata sandi keystore belum dikonfigurasi}"
: "${KEY_ALIAS:?Alias kunci belum dikonfigurasi}"
: "${KEY_PASSWORD:?Kata sandi kunci belum dikonfigurasi}"
: "${ANDROID_HOME:?Android SDK tidak ditemukan}"
: "${RUNNER_TEMP:?Direktori sementara runner tidak ditemukan}"
keyfile="$RUNNER_TEMP/hris-release.keystore"
aligned="$RUNNER_TEMP/hris-release-aligned.apk"
trap 'rm -f "$keyfile" "$aligned"' EXIT
printf '%s' "$KEYSTORE_B64" | base64 --decode > "$keyfile"
tools_dir="$(find "$ANDROID_HOME/build-tools" -mindepth 1 -maxdepth 1 -type d | sort -V | tail -n 1)"
test -x "$tools_dir/zipalign" && test -x "$tools_dir/apksigner"
"$tools_dir/zipalign" -f -p 4 mobile/app/build/outputs/apk/release/app-release-unsigned.apk "$aligned"
"$tools_dir/apksigner" sign --ks "$keyfile" --ks-key-alias "$KEY_ALIAS" \
  --ks-pass env:KEYSTORE_PASSWORD --key-pass env:KEY_PASSWORD \
  --out hris-pejuang-release.apk "$aligned"
"$tools_dir/apksigner" verify --verbose hris-pejuang-release.apk
