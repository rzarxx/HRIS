#!/usr/bin/env bash
# Menguji integritas dan restore arsip backup HRIS terenkripsi ke database sementara (lalu menghapusnya).
# Tidak menyentuh database produksi. Dipakai:
#   bash /opt/hris/src/backend/deploy/restore-verify.sh [path-arsip] [--archive-only]
set -euo pipefail
umask 077

APP_DIR="${APP_DIR:-/opt/hris}"
ENV_FILE="$APP_DIR/.env"
BACKUP_DIR="${BACKUP_DIR:-$APP_DIR/backups}"
BACKUP_PASS_FILE="${BACKUP_PASS_FILE:-/etc/hris/backup.pass}"
RESTORE_TEST_ADMIN_URL="${RESTORE_TEST_ADMIN_URL:-}"

ARCHIVE=""
ARCHIVE_ONLY=0
for arg in "$@"; do
  case "$arg" in
    --archive-only) ARCHIVE_ONLY=1 ;;
    *) ARCHIVE="$arg" ;;
  esac
done

[ "$(id -u)" -eq 0 ] || { echo "Jalankan sebagai root." >&2; exit 1; }
for bin in openssl tar sha256sum pg_restore psql; do
  command -v "$bin" >/dev/null || { echo "Perintah wajib tidak ditemukan: $bin" >&2; exit 1; }
done
[ -f "$BACKUP_PASS_FILE" ] || { echo "Kunci enkripsi backup tidak ditemukan: $BACKUP_PASS_FILE" >&2; exit 1; }

if [ -z "$ARCHIVE" ]; then
  ARCHIVE="$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'hris-backup-*.tar.gz.enc' | LC_ALL=C sort | tail -n 1)"
fi
[ -n "$ARCHIVE" ] && [ -f "$ARCHIVE" ] || { echo "Tidak ada arsip backup di $BACKUP_DIR." >&2; exit 1; }

if [ -f "$ARCHIVE.sha256" ]; then
  sha256sum -c --status "$ARCHIVE.sha256" || { echo "Checksum arsip tidak cocok: $ARCHIVE" >&2; exit 1; }
fi

work="$(mktemp -d)"
temp_db=""
admin_url="$RESTORE_TEST_ADMIN_URL"

cleanup() {
  if [ -n "$temp_db" ] && [ -n "$admin_url" ]; then
    psql "$admin_url" -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS \"$temp_db\";" >/dev/null 2>&1 || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT

echo "==> Mendekripsi dan memverifikasi $ARCHIVE"
openssl enc -d -aes-256-cbc -pbkdf2 -iter 600000 -pass "file:$BACKUP_PASS_FILE" -in "$ARCHIVE" | \
  tar -xzf - -C "$work"

(
  cd "$work"
  sha256sum -c --status manifest.sha256
)
[ -f "$work/db.dump" ] || { echo "db.dump hilang dari arsip." >&2; exit 1; }
[ -f "$work/secrets/jwt-ed25519.key" ] || { echo "jwt-ed25519.key hilang dari arsip." >&2; exit 1; }
pg_restore --list "$work/db.dump" >/dev/null
echo "==> Verifikasi checksum, secrets, dan daftar isi pg_restore OK"

if [ "$ARCHIVE_ONLY" -eq 1 ]; then
  exit 0
fi

if [ -z "$admin_url" ] && [ -f "$ENV_FILE" ]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
  base_url="${MIGRATE_DATABASE_URL:-}"
  if [ -n "$base_url" ]; then
    # Ganti nama database pada URI menjadi postgres untuk membuat/menghapus DB sementara.
    admin_url="$(echo "$base_url" | sed -E 's#(postgres(ql)?://[^/]+/)[^?]+#\1postgres#')"
  fi
fi
[ -n "$admin_url" ] || { echo "RESTORE_TEST_ADMIN_URL atau MIGRATE_DATABASE_URL wajib ada untuk uji restore." >&2; exit 1; }

temp_db="hris_restore_verify_$$"
echo "==> Menguji pg_restore ke database sementara $temp_db"
psql "$admin_url" -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"$temp_db\";" >/dev/null
restore_url="$(echo "$admin_url" | sed -E "s#(postgres(ql)?://[^/]+/)[^?]+#\1$temp_db#")"

pg_restore --no-owner --no-acl --exit-on-error -d "$restore_url" "$work/db.dump"
counts="$(psql "$restore_url" -At -v ON_ERROR_STOP=1 -c "
  SELECT 'schema_migrations=' || COALESCE((SELECT max(version)::text FROM schema_migrations), '0')
      || ' divisions=' || (SELECT count(*) FROM divisions)
      || ' employees=' || (SELECT count(*) FROM employees)
      || ' users=' || (SELECT count(*) FROM users)
      || ' attendances=' || (SELECT count(*) FROM attendances)
      || ' audit_logs=' || (SELECT count(*) FROM audit_logs);
")"
echo "==> Uji restore sukses: $counts"
