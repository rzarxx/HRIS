#!/usr/bin/env bash
# Menyiapkan server development sekali di awal. Aman dijalankan berulang: kata sandi
# yang sudah ada di .env dipakai kembali. Langkah lengkap ada di backend/deploy/README.md.
# COOKIE_DOMAIN wajib diisi domain induk bila web dan API beda subdomain; kosongkan bila memakai IP.
# Skrip menanyakan kata sandi superuser "postgres"; kata sandi itu tidak disimpan.
set -euo pipefail

PSQL="${PSQL:-psql}"
WEB_ORIGIN="${WEB_ORIGIN:?Isi WEB_ORIGIN, mis. https://admin.domain.id atau http://IP:PORT}"
COOKIE_DOMAIN="${COOKIE_DOMAIN:-}"
NEXT_PUBLIC_API_URL="${NEXT_PUBLIC_API_URL:?Isi NEXT_PUBLIC_API_URL, mis. https://api.domain.id}"
NODE_BIN_DIR="${NODE_BIN_DIR:-}"
GO_BIN_DIR="${GO_BIN_DIR:-/usr/local/go/bin}"
BRANCH="${BRANCH:-develop}"
REPO_URL="${REPO_URL:-https://github.com/rzarxx/HRIS.git}"
APP_DIR=/opt/hris
ENV_FILE="$APP_DIR/.env"
SRC_DIR="$APP_DIR/src"
HERE="$(cd "$(dirname "$0")" && pwd)"

id hris >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin hris
# 755 agar hris-build bisa masuk ke src; rahasia dilindungi mode 600/700 pada filenya.
install -d -o hris -g hris -m 755 "$APP_DIR"
install -d -o hris -g hris -m 700 "$APP_DIR/.secrets"

# hris-build menjalankan build tanpa hak root dan tidak bisa membaca rahasia.
id hris-build >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin hris-build
install -d -o hris-build -g hris-build -m 755 "$APP_DIR/build-home"
if [ ! -d "$SRC_DIR/.git" ]; then
  install -d -o hris-build -g hris-build -m 755 "$SRC_DIR"
  runuser -u hris-build -- env HOME="$APP_DIR/build-home" git clone --branch "$BRANCH" "$REPO_URL" "$SRC_DIR"
fi

read_env() { [ -f "$ENV_FILE" ] && grep -E "^$1=" "$ENV_FILE" | head -1 | cut -d= -f2- || true; }
dsn_password() { sed -nE 's#^.*://[^:]+:([^@]+)@.*$#\1#p'; }

HRIS_MIGRATE_PW="$(read_env MIGRATE_DATABASE_URL | dsn_password)"
HRIS_APP_PW="$(read_env DATABASE_URL | dsn_password)"
HRIS_RO_PW="$(read_env HRIS_RO_PASSWORD)"
SEED_PW="$(read_env SEED_SDM_PASSWORD)"
: "${HRIS_MIGRATE_PW:=$(openssl rand -hex 24)}"
: "${HRIS_APP_PW:=$(openssl rand -hex 24)}"
: "${HRIS_RO_PW:=$(openssl rand -hex 24)}"
: "${SEED_PW:=$(openssl rand -hex 16)}"
export HRIS_MIGRATE_PW HRIS_APP_PW HRIS_RO_PW

"$PSQL" -h 127.0.0.1 -U postgres -d postgres -f "$HERE/../scripts/dev-db-setup.sql"

COOKIE_SECURE=false
case "$WEB_ORIGIN" in https://*) COOKIE_SECURE=true ;; esac

umask 077
cat > "$ENV_FILE" <<EOF
APP_ENV=development
HTTP_ADDR=127.0.0.1:8080
DATABASE_URL=postgres://hris_app:$HRIS_APP_PW@127.0.0.1:5432/hris_dev?sslmode=disable
MIGRATE_DATABASE_URL=postgres://hris_migrate:$HRIS_MIGRATE_PW@127.0.0.1:5432/hris_dev?sslmode=disable
HRIS_RO_PASSWORD=$HRIS_RO_PW
SEED_SDM_PASSWORD=$SEED_PW
JWT_SIGNING_KEY_PATH=.secrets/jwt_ed25519.pem
JWT_KEY_ID=dev-1
FIELD_ENCRYPTION_KEY_PATH=.secrets/field_encryption.key
MFA_REQUIRED=false
COOKIE_DOMAIN=$COOKIE_DOMAIN
COOKIE_SECURE=$COOKIE_SECURE
TRUST_PROXY=true
CORS_ALLOWED_ORIGINS=$WEB_ORIGIN
LOG_LEVEL=info
DEV_SIMULATE_ATTENDANCE=true
EOF
chown hris:hris "$ENV_FILE"
chmod 600 "$ENV_FILE"

cat > "$APP_DIR/deploy.env" <<EOF
BRANCH=$BRANCH
NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
NEXT_PUBLIC_DEMO_MODE=1
GO_BIN_DIR=$GO_BIN_DIR
NODE_BIN_DIR=$NODE_BIN_DIR
WEB_ROOT=/www/wwwroot/hris-web
EOF
chmod 600 "$APP_DIR/deploy.env"

install -m 644 "$HERE/hris-api.service" /etc/systemd/system/hris-api.service
systemctl daemon-reload
systemctl enable hris-api

echo "Selesai. Lanjutkan dengan: bash $SRC_DIR/backend/deploy/deploy.sh"
