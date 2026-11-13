#!/usr/bin/env bash
# Backup terenkripsi database PostgreSQL, .env, .secrets, dan folder selfie HRIS Pejuang.
# Dijalankan otomatis oleh hris-backup.timer atau manual sebagai root:
#   bash /opt/hris/src/backend/deploy/backup.sh
set -euo pipefail
umask 077

APP_DIR="${APP_DIR:-/opt/hris}"
ENV_FILE="$APP_DIR/.env"
DEPLOY_ENV="$APP_DIR/deploy.env"
BACKUP_DIR="${BACKUP_DIR:-$APP_DIR/backups}"
BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
BACKUP_PASS_FILE="${BACKUP_PASS_FILE:-/etc/hris/backup.pass}"
BACKUP_REMOTE_DEST="${BACKUP_REMOTE_DEST:-}"

[ "$(id -u)" -eq 0 ] || { echo "Jalankan sebagai root." >&2; exit 1; }
for bin in pg_dump openssl tar sha256sum find; do
  command -v "$bin" >/dev/null || { echo "Perintah wajib tidak ditemukan: $bin" >&2; exit 1; }
done
[ -f "$ENV_FILE" ] || { echo "Berkas $ENV_FILE tidak ditemukan." >&2; exit 1; }

# Muat konfigurasi tanpa membocorkan nilai rahasia ke stdout.
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
if [ -f "$DEPLOY_ENV" ]; then
  # shellcheck disable=SC1090
  source "$DEPLOY_ENV"
fi
set +a

DB_URL="${MIGRATE_DATABASE_URL:-${DATABASE_URL:-}}"
[ -n "$DB_URL" ] || { echo "MIGRATE_DATABASE_URL atau DATABASE_URL wajib diisi di $ENV_FILE." >&2; exit 1; }

install -d -m 700 "$(dirname "$BACKUP_PASS_FILE")" "$BACKUP_DIR"
if [ ! -s "$BACKUP_PASS_FILE" ]; then
  openssl rand -base64 36 > "$BACKUP_PASS_FILE"
  chmod 600 "$BACKUP_PASS_FILE"
  echo "PENTING: Kunci enkripsi backup baru dibuat di $BACKUP_PASS_FILE." >&2
  echo "PENTING: Salin berkas tersebut ke pengelola sandi / vault di luar server!" >&2
fi

ts="$(date -u +%Y%m%dT%H%M%SZ)"
work="$(mktemp -d)"
cleanup() {
  rm -rf "$work"
}
trap cleanup EXIT

echo "==> Dump database PostgreSQL"
pg_dump --format=custom --no-owner --no-acl -f "$work/db.dump" "$DB_URL"

mkdir -p "$work/config"
cp -a "$ENV_FILE" "$work/config/.env"
if [ -f "$DEPLOY_ENV" ]; then
  cp -a "$DEPLOY_ENV" "$work/config/deploy.env"
fi
if [ -d "$APP_DIR/.secrets" ]; then
  cp -a "$APP_DIR/.secrets" "$work/secrets"
fi
if [ -d "$APP_DIR/selfies" ]; then
  cp -a "$APP_DIR/selfies" "$work/selfies"
fi

{
  echo "created_at_utc=$ts"
  echo "hostname=$(hostname -f 2>/dev/null || hostname)"
  if [ -d "$APP_DIR/src/.git" ]; then
    echo "git_commit=$(git -C "$APP_DIR/src" rev-parse --short HEAD 2>/dev/null || echo unknown)"
  fi
} > "$work/manifest.txt"

(
  cd "$work"
  find . -type f ! -name manifest.sha256 | LC_ALL=C sort | xargs sha256sum > "$work/manifest.sha256"
)

archive="$BACKUP_DIR/hris-backup-$ts.tar.gz.enc"
tar -czf - -C "$work" . | \
  openssl enc -aes-256-cbc -pbkdf2 -iter 600000 -salt -pass "file:$BACKUP_PASS_FILE" -out "$archive.tmp"
mv -f "$archive.tmp" "$archive"
chmod 600 "$archive"
sha256sum "$archive" > "$archive.sha256"
chmod 600 "$archive.sha256"

echo "==> Backup terenkripsi tersimpan: $archive"

# Bersihkan arsip lokal melebihi masa simpan.
find "$BACKUP_DIR" -maxdepth 1 -type f \( -name 'hris-backup-*.tar.gz.enc' -o -name 'hris-backup-*.tar.gz.enc.sha256' \) \
  -mtime +"$BACKUP_RETENTION_DAYS" -delete

# Sinkronisasi opsional ke penyimpanan di luar server.
if [ -n "$BACKUP_REMOTE_DEST" ]; then
  case "$BACKUP_REMOTE_DEST" in
    rclone://*)
      command -v rclone >/dev/null || { echo "rclone belum terpasang untuk $BACKUP_REMOTE_DEST" >&2; exit 1; }
      dest="${BACKUP_REMOTE_DEST#rclone://}"
      rclone copy "$archive" "$dest"
      rclone copy "$archive.sha256" "$dest"
      echo "==> Tersalin ke rclone: $dest"
      ;;
    *)
      command -v rsync >/dev/null || { echo "rsync belum terpasang untuk $BACKUP_REMOTE_DEST" >&2; exit 1; }
      rsync -az "$archive" "$archive.sha256" "$BACKUP_REMOTE_DEST"
      echo "==> Tersalin ke tujuan jarak jauh: $BACKUP_REMOTE_DEST"
      ;;
  esac
else
  echo "PERHATIAN: BACKUP_REMOTE_DEST belum disetel; salin $archive ke luar server agar tahan kegagalan VPS." >&2
fi
