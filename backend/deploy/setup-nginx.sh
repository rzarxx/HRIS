#!/usr/bin/env bash
# Memasang konfigurasi Nginx ke situs aaPanel. Jalankan sebagai root setelah kedua situs dibuat:
#   API_SITE=api.hris.local WEB_SITE=admin.hris.local bash setup-nginx.sh
# Aman dijalankan ulang. Cadangan tersimpan sebagai <situs>.conf.bak-hris, dan otomatis
# dikembalikan bila nginx -t gagal.
#
# Konfigurasi disalin ke folder milik root, bukan di-include dari repo, karena Nginx berjalan
# sebagai root dan repo bisa ditulis oleh user hris-build.
set -euo pipefail

API_SITE="${API_SITE:?Isi API_SITE, mis. api.hris.local}"
WEB_SITE="${WEB_SITE:?Isi WEB_SITE, mis. admin.hris.local}"
NGINX=/www/server/nginx/sbin/nginx
VHOST_DIR=/www/server/panel/vhost/nginx
INC_DIR=/www/server/nginx/conf
HERE="$(cd "$(dirname "$0")" && pwd)"

install_site() {
  local site="$1" src="$2" inc="$3"
  local conf="$VHOST_DIR/$site.conf"
  [ -f "$conf" ] || { echo "$site: tidak ada $conf, buat situsnya dulu di aaPanel"; return 1; }
  install -m 644 "$HERE/$src" "$INC_DIR/$inc"
  if grep -q "$inc" "$conf"; then
    echo "$site: konfigurasi diperbarui"
    return 0
  fi
  if grep -qE '^\s*location\s+/\s*\{' "$conf"; then
    echo "$site: sudah ada location /, hapus dulu blok itu"
    return 1
  fi
  cp "$conf" "$conf.bak-hris"
  # Blok statis bawaan aaPanel menangkap file sebelum aturan kita.
  sed -i '/location ~ \.\*\\\.(gif|jpg/,/^\s*}/d; /location ~ \.\*\\\.(js|css)/,/^\s*}/d' "$conf"
  sed -i "0,/location ~ \^\/(\\\\.user/s||include $INC_DIR/$inc;\n    &|" "$conf"
  if ! grep -q "$inc" "$conf" || ! "$NGINX" -t >/dev/null 2>&1; then
    cp "$conf.bak-hris" "$conf"
    echo "$site: gagal, cadangan dikembalikan"
    "$NGINX" -t || true
    return 1
  fi
  echo "$site: konfigurasi terpasang"
}

install_site "$API_SITE" nginx-api.conf hris-api.inc
install_site "$WEB_SITE" nginx-web.conf hris-web.inc
"$NGINX" -t
"$NGINX" -s reload
echo "Nginx dimuat ulang."
