#!/usr/bin/env bash
# Memperbarui server development dari Git: pull, build, migrasi, restart.
# Jalankan sebagai root setelah CI branch develop hijau:
#   bash /opt/hris/src/backend/deploy/deploy.sh
#
# Build berjalan sebagai user hris-build tanpa hak root, karena dependensi Go dan npm
# adalah kode pihak ketiga. Bila build gagal, versi yang sedang berjalan tidak tersentuh.
set -euo pipefail

APP_DIR=/opt/hris
SRC_DIR="$APP_DIR/src"
CONFIG="$APP_DIR/deploy.env"

[ "$(id -u)" -eq 0 ] || { echo "Jalankan sebagai root."; exit 1; }
[ -f "$CONFIG" ] || { echo "$CONFIG belum ada, jalankan setup-server.sh dulu."; exit 1; }
# shellcheck disable=SC1090
source "$CONFIG"
: "${BRANCH:=develop}"
: "${WEB_ROOT:=/www/wwwroot/hris-web}"
: "${GO_BIN_DIR:=/usr/local/go/bin}"
: "${NEXT_PUBLIC_API_URL:?Isi NEXT_PUBLIC_API_URL di $CONFIG}"
: "${NEXT_PUBLIC_DEMO_MODE:=1}"
# Batas memori build Next.js; kecilkan di server ber-RAM kecil.
: "${NODE_MAX_OLD_SPACE:=1536}"

BUILD_PATH="$GO_BIN_DIR:${NODE_BIN_DIR:+$NODE_BIN_DIR:}/usr/local/bin:/usr/bin:/bin"
as_builder() {
  runuser -u hris-build -- env \
    HOME="$APP_DIR/build-home" PATH="$BUILD_PATH" CGO_ENABLED=0 \
    NEXT_PUBLIC_API_URL="$NEXT_PUBLIC_API_URL" NEXT_PUBLIC_DEMO_MODE="$NEXT_PUBLIC_DEMO_MODE" \
    NODE_OPTIONS="--max-old-space-size=$NODE_MAX_OLD_SPACE" NEXT_TELEMETRY_DISABLED=1 \
    "$@"
}

echo "==> Mengambil $BRANCH"
as_builder git -C "$SRC_DIR" fetch --prune origin "$BRANCH"
as_builder git -C "$SRC_DIR" checkout -q "$BRANCH"
# Hanya maju: bila riwayat bercabang atau ada perubahan lokal, berhenti daripada menimpa.
as_builder git -C "$SRC_DIR" merge --ff-only "origin/$BRANCH"
COMMIT="$(as_builder git -C "$SRC_DIR" log -1 --format='%h %s')"
echo "    $COMMIT"

echo "==> Build API"
BUILD_DIR="$APP_DIR/build-home/dist"
as_builder mkdir -p "$BUILD_DIR"
for program in migrate seed superadmin; do
  as_builder bash -c "cd '$SRC_DIR/backend' && go build -o '$BUILD_DIR/hris-$program' ./cmd/$program"
done
# Mesin wajah dlib butuh cgo dan pustaka dari setup-face.sh (D42). Tanpanya API tetap jalan, tetapi
# daftar wajah dan absen dengan selfie dijawab FACE_SERVICE_UNAVAILABLE.
if [ "${FACE_ENGINE:-}" = "dlib" ]; then
  echo "    API dengan mesin wajah dlib (build pertama bisa beberapa menit)"
  as_builder env CGO_ENABLED=1 bash -c "cd '$SRC_DIR/backend' && go build -tags dlib -o '$BUILD_DIR/hris-api' ./cmd/api"
else
  as_builder bash -c "cd '$SRC_DIR/backend' && go build -o '$BUILD_DIR/hris-api' ./cmd/api"
fi

echo "==> Build web"
as_builder bash -c "cd '$SRC_DIR/web' && npm ci --ignore-scripts --no-audit --no-fund && npm run build"

echo "==> Memasang"
for program in api migrate seed superadmin; do
  install -o hris -g hris -m 750 "$BUILD_DIR/hris-$program" "$APP_DIR/hris-$program.new"
  mv -f "$APP_DIR/hris-$program.new" "$APP_DIR/hris-$program"
done
rm -rf "$APP_DIR/migrations"
cp -r "$SRC_DIR/backend/migrations" "$APP_DIR/migrations"
chown -R hris:hris "$APP_DIR/migrations"

echo "==> Migrasi"
(cd "$APP_DIR" && runuser -u hris -- ./hris-migrate up)

echo "==> Restart API"
systemctl restart hris-api
for _ in $(seq 1 15); do
  curl -fs -o /dev/null http://127.0.0.1:8080/readyz && break
  sleep 1
done
curl -fsS -o /dev/null http://127.0.0.1:8080/readyz || {
  echo "API tidak siap. Log terakhir:"
  journalctl -u hris-api -n 30 --no-pager
  exit 1
}

echo "==> Menyalin web ke $WEB_ROOT"
# Web disalin terakhir agar tidak mendahului API. File milik aaPanel tidak ikut dihapus.
rsync -a --delete --chown=www:www \
  --exclude .user.ini --exclude .htaccess --exclude .well-known \
  "$SRC_DIR/web/out/" "$WEB_ROOT/"

echo "Selesai: $COMMIT"
