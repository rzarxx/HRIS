#!/usr/bin/env bash
# Mengalihkan konfigurasi VPS HRIS Pejuang ke mode produksi, memeriksa TLS HTTPS,
# membangun biner hris-bootstrap, dan mengaktifkan jadwal backup + uji restore.
# Jalankan sebagai root setelah setup-server.sh dan setup-nginx.sh:
#   bash /opt/hris/src/backend/deploy/setup-production.sh [--allow-no-tls]
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/hris}"
ENV_FILE="$APP_DIR/.env"
CONFIG="$APP_DIR/deploy.env"
HERE="$(cd "$(dirname "$0")" && pwd)"
ALLOW_NO_TLS=0

for arg in "$@"; do
  case "$arg" in
    --allow-no-tls) ALLOW_NO_TLS=1 ;;
    *) echo "Argumen tidak dikenal: $arg" >&2; exit 1 ;;
  esac
done

[ "$(id -u)" -eq 0 ] || { echo "Jalankan sebagai root." >&2; exit 1; }
[ -f "$ENV_FILE" ] && [ -f "$CONFIG" ] || { echo "Jalankan setup-server.sh dulu." >&2; exit 1; }

if [ "$ALLOW_NO_TLS" -eq 0 ]; then
  for domain in hris.albahjah.or.id api.albahjah.or.id; do
    if [ ! -f "/etc/letsencrypt/live/$domain/fullchain.pem" ]; then
      echo "Sertifikat TLS untuk $domain belum ditemukan. Jalankan setup-nginx.sh / certbot dulu, atau pakai --allow-no-tls jika SSL dihentikan di aaPanel." >&2
      exit 1
    fi
  done
fi

set_key() {
  local file="$1" key="$2" value="$3"
  if grep -qE "^$key=" "$file"; then
    sed -i "s#^$key=.*#$key=$value#" "$file"
  else
    echo "$key=$value" >> "$file"
  fi
}

set_key "$ENV_FILE" APP_ENV production
set_key "$ENV_FILE" MFA_REQUIRED true
set_key "$ENV_FILE" COOKIE_SECURE true
set_key "$ENV_FILE" COOKIE_DOMAIN "${COOKIE_DOMAIN:-albahjah.or.id}"
set_key "$ENV_FILE" WEB_ORIGIN "${WEB_ORIGIN:-https://hris.albahjah.or.id}"
set_key "$ENV_FILE" DEV_ALLOW_UNVERIFIED_ATTENDANCE false
set_key "$ENV_FILE" DEV_SIMULATE_ATTENDANCE false
chown hris:hris "$ENV_FILE"
chmod 600 "$ENV_FILE"

set_key "$CONFIG" NEXT_PUBLIC_API_URL "${NEXT_PUBLIC_API_URL:-https://api.albahjah.or.id}"
set_key "$CONFIG" NEXT_PUBLIC_DEMO_MODE 0
set_key "$CONFIG" WEB_ROOT "${WEB_ROOT:-/www/wwwroot/HRIS}"
chmod 600 "$CONFIG"

echo "==> Membangun hris-bootstrap"
(
  cd "$APP_DIR/src/backend"
  go build -trimpath -o "$APP_DIR/hris-bootstrap" ./cmd/bootstrap
)
chown root:hris "$APP_DIR/hris-bootstrap"
chmod 750 "$APP_DIR/hris-bootstrap"

echo "==> Memasang timer systemd backup harian & uji restore mingguan"
install -m 644 "$HERE/hris-backup.service" /etc/systemd/system/hris-backup.service
install -m 644 "$HERE/hris-backup.timer" /etc/systemd/system/hris-backup.timer
install -m 644 "$HERE/hris-restore-verify.service" /etc/systemd/system/hris-restore-verify.service
install -m 644 "$HERE/hris-restore-verify.timer" /etc/systemd/system/hris-restore-verify.timer
systemctl daemon-reload
systemctl enable --now hris-backup.timer hris-restore-verify.timer

echo "Selesai. Langkah berikut:"
echo "  1. Pastikan JWT_SIGNING_KEY_PATH & FIELD_ENCRYPTION_KEY terisi aman dan backup /etc/hris/backup.pass setelah backup pertama."
echo "  2. Jalankan deploy: bash $HERE/deploy.sh"
echo "  3. Bootstrap akun SDM pertama (jika DB masih kosong):"
echo "     BOOTSTRAP_SDM_PASSWORD='...' $APP_DIR/hris-bootstrap -email admin.sdm@albahjah.or.id"
echo "  4. Buat backup awal + uji restore:"
echo "     bash $HERE/backup.sh && bash $HERE/restore-verify.sh"
