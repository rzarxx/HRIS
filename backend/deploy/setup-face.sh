#!/usr/bin/env bash
# Menyiapkan verifikasi wajah di server (D43): ONNX Runtime dengan detektor YuNet dan pengenal
# SFace. Jalankan sebagai root SEBELUM deploy.sh, lalu jalankan deploy.sh seperti biasa:
#   bash /opt/hris/src/backend/deploy/setup-face.sh
# Aman dijalankan berulang.
#
# Yang dilakukan: mengunduh ONNX Runtime dan dua model dengan checksum, membuat folder selfie,
# menghapus model dlib lama (D42), dan mengisi FACE_MODELS_DIR, ONNXRUNTIME_LIB, SELFIE_DIR, dan
# FACE_ENGINE=onnx.
set -euo pipefail

APP_DIR=/opt/hris
ENV_FILE="$APP_DIR/.env"
CONFIG="$APP_DIR/deploy.env"
MODELS_DIR="$APP_DIR/face-models"
ORT_DIR="$APP_DIR/onnxruntime"
SELFIE_DIR="$APP_DIR/selfies"
HERE="$(cd "$(dirname "$0")" && pwd)"

# ONNX Runtime harus sama dengan versi header github.com/yalue/onnxruntime_go di go.mod.
ORT_VERSION=1.31.0
ORT_URL="https://github.com/microsoft/onnxruntime/releases/download/v$ORT_VERSION/onnxruntime-linux-x64-$ORT_VERSION.tgz"
ORT_SHA256=cc5c72baf5ae5c8238a6841f0897227be2d02826b9cf98eaf02fdefaeeb03a57
ORT_LIB="$ORT_DIR/lib/libonnxruntime.so.$ORT_VERSION"

# Model OpenCV Zoo pada commit yang dikunci: YuNet (MIT) dan SFace (Apache-2.0).
ZOO=https://media.githubusercontent.com/media/opencv/opencv_zoo/48b10fa505aeb51fbd127fc60a5c660040e823b7/models
declare -A MODELS=(
  [yunet.onnx]="$ZOO/face_detection_yunet/face_detection_yunet_2023mar.onnx 8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4"
  [sface.onnx]="$ZOO/face_recognition_sface/face_recognition_sface_2021dec.onnx 0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79"
)

[ "$(id -u)" -eq 0 ] || { echo "Jalankan sebagai root."; exit 1; }
[ -f "$ENV_FILE" ] && [ -f "$CONFIG" ] || { echo "Jalankan setup-server.sh dulu."; exit 1; }
command -v gcc >/dev/null || { echo "gcc belum ada. Pasang build-essential dulu (dibutuhkan cgo)."; exit 1; }

echo "==> ONNX Runtime $ORT_VERSION di $ORT_DIR"
if [ ! -f "$ORT_LIB" ]; then
  tmp="$(mktemp -d)"
  curl -fsSL -o "$tmp/ort.tgz" "$ORT_URL"
  echo "$ORT_SHA256  $tmp/ort.tgz" | sha256sum -c --status || { rm -rf "$tmp"; echo "Checksum ONNX Runtime tidak cocok."; exit 1; }
  rm -rf "$ORT_DIR"
  install -d -m 755 "$ORT_DIR"
  tar -xzf "$tmp/ort.tgz" -C "$ORT_DIR" --strip-components=1
  rm -rf "$tmp"
fi
chown -R root:root "$ORT_DIR"
chmod -R a+rX,go-w "$ORT_DIR"

echo "==> Model wajah di $MODELS_DIR"
install -d -o root -g hris -m 750 "$MODELS_DIR"
for name in "${!MODELS[@]}"; do
  read -r url sum <<<"${MODELS[$name]}"
  target="$MODELS_DIR/$name"
  if ! echo "$sum  $target" | sha256sum -c --status 2>/dev/null; then
    curl -fsSL -o "$target.tmp" "$url"
    echo "$sum  $target.tmp" | sha256sum -c --status || { rm -f "$target.tmp"; echo "Checksum $name tidak cocok."; exit 1; }
    mv -f "$target.tmp" "$target"
  fi
  chown root:hris "$target"
  chmod 640 "$target"
done
# Model dlib (D42) tidak dipakai lagi.
rm -f "$MODELS_DIR"/*.dat

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
set_key "$ENV_FILE" ONNXRUNTIME_LIB "$ORT_LIB"
set_key "$ENV_FILE" SELFIE_DIR "$SELFIE_DIR"
set_key "$CONFIG" FACE_ENGINE onnx
chown hris:hris "$ENV_FILE"
chmod 600 "$ENV_FILE" "$CONFIG"

# Layanan perlu izin menulis ke folder selfie.
install -m 644 "$HERE/hris-api.service" /etc/systemd/system/hris-api.service
systemctl daemon-reload

echo "Selesai. Lanjutkan dengan: bash $APP_DIR/src/backend/deploy/deploy.sh"
echo "Paket dlib dari langkah sebelumnya tidak dipakai lagi; boleh dihapus dengan: apt-get remove libdlib-dev libdlib19 libdlib-data"
