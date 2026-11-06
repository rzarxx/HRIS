#!/usr/bin/env bash
# Menyiapkan verifikasi wajah di server (D42), sekali saja. Jalankan sebagai root:
#   bash /opt/hris/src/backend/deploy/setup-face.sh
# lalu jalankan deploy.sh seperti biasa. Aman dijalankan berulang.
#
# Yang dilakukan: memasang pustaka dlib untuk build, mengunduh model dlib dan mencocokkan
# checksum-nya, membuat folder selfie, dan mengisi FACE_MODELS_DIR, SELFIE_DIR, dan FACE_ENGINE.
set -euo pipefail

APP_DIR=/opt/hris
ENV_FILE="$APP_DIR/.env"
CONFIG="$APP_DIR/deploy.env"
MODELS_DIR="$APP_DIR/face-models"
SELFIE_DIR="$APP_DIR/selfies"
HERE="$(cd "$(dirname "$0")" && pwd)"
# Model dari github.com/Kagami/go-face-testdata pada commit yang dikunci. Model dlib dirilis
# pembuatnya ke domain publik (github.com/davisking/dlib-models).
MODELS_URL=https://github.com/Kagami/go-face-testdata/raw/96ccafa591055ef57286ffed1c3d7e39d17140b7/models
declare -A MODELS=(
  [shape_predictor_5_face_landmarks.dat]=c4b1e9804792707d3a405c2c16a80a20269e6675021f64a41d30fffafbc41888
  [dlib_face_recognition_resnet_model_v1.dat]=55533b28a95800a551ba546ba62fe69625c7e95a7061c338adffead08719da30
)

[ "$(id -u)" -eq 0 ] || { echo "Jalankan sebagai root."; exit 1; }
[ -f "$ENV_FILE" ] && [ -f "$CONFIG" ] || { echo "Jalankan setup-server.sh dulu."; exit 1; }

echo "==> Pustaka build dlib"
if command -v apt-get >/dev/null; then
  apt-get update -q
  # Nama paket libjpeg berbeda antara Ubuntu dan Debian.
  JPEG=libjpeg-turbo8-dev
  apt-cache show "$JPEG" >/dev/null 2>&1 || JPEG=libjpeg62-turbo-dev
  DEBIAN_FRONTEND=noninteractive apt-get install -y -q build-essential libdlib-dev libblas-dev libatlas-base-dev liblapack-dev "$JPEG"
else
  echo "Hanya apt (Ubuntu/Debian) yang didukung skrip ini. Pasang dlib, BLAS, LAPACK, dan libjpeg secara manual."
  exit 1
fi

echo "==> Model wajah di $MODELS_DIR"
install -d -o root -g hris -m 750 "$MODELS_DIR"
for name in "${!MODELS[@]}"; do
  target="$MODELS_DIR/$name"
  if ! echo "${MODELS[$name]}  $target" | sha256sum -c --status 2>/dev/null; then
    curl -fsSL -o "$target.tmp" "$MODELS_URL/$name"
    echo "${MODELS[$name]}  $target.tmp" | sha256sum -c --status || { rm -f "$target.tmp"; echo "Checksum $name tidak cocok."; exit 1; }
    mv -f "$target.tmp" "$target"
  fi
  chown root:hris "$target"
  chmod 640 "$target"
done

echo "==> Folder selfie di $SELFIE_DIR"
install -d -o hris -g hris -m 700 "$SELFIE_DIR"

set_key() {
  local file="$1" key="$2" value="$3"
  if grep -qE "^$key=" "$file"; then
    sed -i "s#^$key=.*#$key=$value#" "$file"
  else
    echo "$key=$value" >> "$file"
  fi
}
set_key "$ENV_FILE" FACE_MODELS_DIR "$MODELS_DIR"
set_key "$ENV_FILE" SELFIE_DIR "$SELFIE_DIR"
set_key "$CONFIG" FACE_ENGINE dlib
chown hris:hris "$ENV_FILE"
chmod 600 "$ENV_FILE" "$CONFIG"

# Layanan perlu izin menulis ke folder selfie.
install -m 644 "$HERE/hris-api.service" /etc/systemd/system/hris-api.service
systemctl daemon-reload

echo "Selesai. Lanjutkan dengan: bash $APP_DIR/src/backend/deploy/deploy.sh"
