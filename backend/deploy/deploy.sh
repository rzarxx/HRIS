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
SELF="$SRC_DIR/backend/deploy/deploy.sh"
# Hash isi skrip yang sedang berjalan, sebelum git memperbaruinya.
SELF_SUM="$(sha256sum "$SELF" 2>/dev/null | cut -d" " -f1)"
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
# Bash tetap menjalankan isi skrip lama setelah git mengganti berkasnya. Bila skrip ini ikut berubah,
# jalankan ulang versi barunya sekali agar langkah baru tidak terlewat.
if [ -z "${HRIS_DEPLOY_REEXEC:-}" ] && [ "$(sha256sum "$SELF" | cut -d" " -f1)" != "$SELF_SUM" ]; then
  echo "    deploy.sh berubah, menjalankan versi terbaru"
  HRIS_DEPLOY_REEXEC=1 exec bash "$SELF" "$@"
fi
COMMIT="$(as_builder git -C "$SRC_DIR" log -1 --format='%h %s')"
echo "    $COMMIT"

echo "==> Build API"
BUILD_DIR="$APP_DIR/build-home/dist"
as_builder mkdir -p "$BUILD_DIR"
for program in migrate seed superadmin purge-selfies mark-absent; do
  as_builder bash -c "cd '$SRC_DIR/backend' && go build -o '$BUILD_DIR/hris-$program' ./cmd/$program"
done
# Mesin wajah ONNX butuh cgo dan pustaka dari setup-face.sh (D43). Tanpanya API tetap jalan, tetapi
# daftar wajah dan absen dengan selfie dijawab FACE_SERVICE_UNAVAILABLE.
case "${FACE_ENGINE:-}" in
  onnx)
    echo "    API dengan mesin wajah ONNX (SFace)"
    as_builder env CGO_ENABLED=1 bash -c "cd '$SRC_DIR/backend' && go build -tags onnx -o '$BUILD_DIR/hris-api' ./cmd/api"
    ;;
  "")
    as_builder bash -c "cd '$SRC_DIR/backend' && go build -o '$BUILD_DIR/hris-api' ./cmd/api"
    ;;
  *)
    # Mis. FACE_ENGINE=dlib dari D42. Berhenti sebelum biner yang berjalan diganti.
    echo "FACE_ENGINE=${FACE_ENGINE} tidak dikenal lagi. Jalankan dulu: bash $SRC_DIR/backend/deploy/setup-face.sh"
    exit 1
    ;;
esac

echo "==> Build web"
as_builder bash -c "cd '$SRC_DIR/web' && npm ci --ignore-scripts --no-audit --no-fund && npm run build"

echo "==> Memasang"
for program in api migrate seed superadmin purge-selfies mark-absent; do
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

echo "==> Jadwal hapus selfie lama"
# Timer harian purge-selfies (retensi 90 hari, MASTER.md §5.6). Dipasang ulang setiap deploy agar
# ikut berubah bersama repo; service-nya sendiri dilewati bila /opt/hris/selfies belum ada.
install -m 644 "$SRC_DIR/backend/deploy/hris-purge-selfies.service" /etc/systemd/system/hris-purge-selfies.service
install -m 644 "$SRC_DIR/backend/deploy/hris-purge-selfies.timer" /etc/systemd/system/hris-purge-selfies.timer
systemctl daemon-reload
systemctl enable --now hris-purge-selfies.timer >/dev/null
systemctl list-timers hris-purge-selfies.timer --no-pager || true

echo "==> Jadwal catat alpha"
# Timer harian mark_absent (MASTER.md §11.3), dipasang ulang setiap deploy seperti purge-selfies.
install -m 644 "$SRC_DIR/backend/deploy/hris-mark-absent.service" /etc/systemd/system/hris-mark-absent.service
install -m 644 "$SRC_DIR/backend/deploy/hris-mark-absent.timer" /etc/systemd/system/hris-mark-absent.timer
systemctl daemon-reload
systemctl enable --now hris-mark-absent.timer >/dev/null
systemctl list-timers hris-mark-absent.timer --no-pager || true

echo "==> Menyalin web ke $WEB_ROOT"
# Web disalin terakhir agar tidak mendahului API. File milik aaPanel tidak ikut dihapus.
rsync -a --delete --chown=www:www \
  --exclude .user.ini --exclude .htaccess --exclude .well-known \
  "$SRC_DIR/web/out/" "$WEB_ROOT/"

echo "Selesai: $COMMIT"
