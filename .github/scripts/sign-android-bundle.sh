#!/usr/bin/env bash
# Menandatangani Android App Bundle rilis dengan kunci unggah (upload key) Play Console.
# Google Play menandatangani ulang APK yang dipasang di HP dengan kunci aplikasi (Play App Signing).
set -euo pipefail
: "${KEYSTORE_B64:?Keystore unggah belum dikonfigurasi}"
: "${KEYSTORE_PASSWORD:?Kata sandi keystore belum dikonfigurasi}"
: "${KEY_ALIAS:?Alias kunci belum dikonfigurasi}"
: "${KEY_PASSWORD:?Kata sandi kunci belum dikonfigurasi}"
: "${RUNNER_TEMP:?Direktori sementara runner tidak ditemukan}"
keyfile="$RUNNER_TEMP/hris-upload.keystore"
trap 'rm -f "$keyfile"' EXIT
printf '%s' "$KEYSTORE_B64" | base64 --decode > "$keyfile"
bundle=mobile/app/build/outputs/bundle/release/app-release.aab
test -f "$bundle"
jarsigner -keystore "$keyfile" -storepass:env KEYSTORE_PASSWORD -keypass:env KEY_PASSWORD \
  -sigalg SHA256withRSA -digestalg SHA-256 -signedjar hris-yayasan-albahjah.aab "$bundle" "$KEY_ALIAS"
# Kunci unggah self-signed tidak punya rantai CA, jadi -strict tanpa keystore selalu gagal (kode 4).
# Verifikasi terhadap keystore unggah sekaligus memastikan bundle ditandatangani kunci ini.
jarsigner -verify -strict -keystore "$keyfile" -storepass:env KEYSTORE_PASSWORD hris-yayasan-albahjah.aab "$KEY_ALIAS"
