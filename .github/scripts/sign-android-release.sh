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
# Isi APK harus sesuai konfigurasi build: hanya pustaka ARM, dan semuanya dikompresi
# (useLegacyPackaging), agar bisa dipasang oleh pemasang bawaan OEM.
# Kolom unzip -v: Length Method Size Cmpr Date Time CRC-32 Name; Method "Stored" berarti tanpa kompresi.
libs="$(unzip -v hris-pejuang-release.apk | awk '$NF ~ /^lib\/.*\.so$/ {print $NF, $2}')"
test -n "$libs" || { echo 'APK tidak berisi pustaka native' >&2; exit 1; }
echo "$libs"
if grep -qv '^lib/\(arm64-v8a\|armeabi-v7a\)/' <<<"$libs"; then echo 'APK berisi pustaka selain ARM' >&2; exit 1; fi
if grep -q ' Stored$' <<<"$libs"; then echo 'APK berisi pustaka native tanpa kompresi' >&2; exit 1; fi
