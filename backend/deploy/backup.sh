#!/usr/bin/env bash
# Backup terenkripsi database PostgreSQL, .env, .secrets, dan berkas privat HRIS Pejuang.
# Dijalankan otomatis oleh hris-backup.timer atau manual sebagai root:
#   bash /opt/hris/src/backend/deploy/backup.sh
set -euo pipefail
umask 077

# PostgreSQL dari aaPanel terpasang di /www/server/pgsql/bin, di luar PATH bawaan systemd dan root.
# Ditaruh di akhir PATH agar pg_dump sistem (bila ada) tetap didahulukan.
for pg_dir in /www/server/pgsql/bin /usr/local/pgsql/bin; do
  [ -d "$pg_dir" ] && PATH="$PATH:$pg_dir"
done

APP_DIR="${APP_DIR:-/opt/hris}"
ENV_FILE="$APP_DIR/.env"
DEPLOY_ENV="$APP_DIR/deploy.env"
BACKUP_DIR="${BACKUP_DIR:-$APP_DIR/backups}"
BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
BACKUP_PASS_FILE="${BACKUP_PASS_FILE:-/etc/hris/backup.pass}"
BACKUP_REMOTE_DEST="${BACKUP_REMOTE_DEST:-}"

[ "$(id -u)" -eq 0 ] || { echo "Jalankan sebagai root." >&2; exit 1; }
for bin in pg_dump psql openssl tar sha256sum find realpath; do
  command -v "$bin" >/dev/null || { echo "Perintah wajib tidak ditemukan: $bin" >&2; exit 1; }
done
[ -f "$ENV_FILE" ] || { echo "Berkas $ENV_FILE tidak ditemukan." >&2; exit 1; }

# Muat konfigurasi tanpa membocorkan nilai rahasia ke stdout.
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
storage_driver="${STORAGE_DRIVER:-disk}"
if [ -f "$DEPLOY_ENV" ]; then
  # shellcheck disable=SC1090
  source "$DEPLOY_ENV"
fi
set +a

case "$storage_driver" in
  disk|s3) ;;
  *) echo "STORAGE_DRIVER tidak dikenal: $storage_driver" >&2; exit 1 ;;
esac

DB_URL="${MIGRATE_DATABASE_URL:-${DATABASE_URL:-}}"
[ -n "$DB_URL" ] || { echo "MIGRATE_DATABASE_URL atau DATABASE_URL wajib diisi di $ENV_FILE." >&2; exit 1; }
# Pengaturan dari panel disimpan di database dan mengalahkan STORAGE_DRIVER lama.
managed_s3="$(psql "$DB_URL" -Atq -v ON_ERROR_STOP=1 -c "SELECT CASE WHEN EXISTS (SELECT 1 FROM app_settings WHERE key = 'storage_s3') THEN 1 ELSE 0 END")"
if [ "$managed_s3" = "1" ]; then
  storage_driver=s3
fi

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

if [ "$storage_driver" = "s3" ]; then
  [ -n "${STORAGE_BACKUP_REMOTE:-}" ] || { echo "STORAGE_BACKUP_REMOTE wajib untuk backup S3." >&2; exit 1; }
  command -v rclone >/dev/null || { echo "rclone wajib untuk backup S3." >&2; exit 1; }
  mkdir -p "$work/objects"
  # Objek memakai kunci unik dan tidak ditimpa. Salin sebelum dan sesudah pg_dump
  # agar objek yang dihapus atau dibuat saat dump tetap tersedia untuk snapshot DB.
  echo "==> Salin objek S3 sebelum snapshot database"
  rclone copy "$STORAGE_BACKUP_REMOTE" "$work/objects"
fi

echo "==> Dump database PostgreSQL"
pg_dump --format=custom --no-owner --no-acl -f "$work/db.dump" "$DB_URL"
managed_s3_after="$(psql "$DB_URL" -Atq -v ON_ERROR_STOP=1 -c "SELECT CASE WHEN EXISTS (SELECT 1 FROM app_settings WHERE key = 'storage_s3') THEN 1 ELSE 0 END")"
[ "$managed_s3" = "$managed_s3_after" ] || { echo "Mode penyimpanan berubah selama backup; ulangi backup agar snapshot lengkap." >&2; exit 1; }

mkdir -p "$work/config"
cp -a "$ENV_FILE" "$work/config/.env"
if [ -f "$DEPLOY_ENV" ]; then
  cp -a "$DEPLOY_ENV" "$work/config/deploy.env"
fi
if [ -d "$APP_DIR/.secrets" ]; then
  cp -a "$APP_DIR/.secrets" "$work/secrets"
fi
if [ "$storage_driver" = "s3" ]; then
  echo "==> Salin objek S3 setelah snapshot database"
  rclone copy "$STORAGE_BACKUP_REMOTE" "$work/objects"
fi
# Salinan disk tetap diperlukan selama migrasi dan sebagai fallback bila objek lama
# belum berada di S3. SELFIE_DIR dapat berbeda dari lokasi default instalasi.
legacy_dir="${SELFIE_DIR:-$APP_DIR/selfies}"
if [ -d "$legacy_dir" ]; then
  legacy_dir="$(realpath -e "$legacy_dir")"
  cp -a "$legacy_dir" "$work/selfies"
fi
printf '%s\n' "$storage_driver" > "$work/storage-mode.txt"

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
