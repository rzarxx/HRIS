# Deploy ke server development (aaPanel + Git)

Server development memakai **data dummy**. Jangan isi data karyawan asli: mode ini mematikan MFA dan mengizinkan seed.
Nama menu dan lokasi folder aaPanel bisa sedikit berbeda antar versi.

```
Browser ─► Nginx aaPanel ─┬─ Development : admin.hris.local (web admin) & api.albahjah.net (API mobile/dev)
                          └─ Production  : hris.albahjah.or.id (/www/wwwroot/HRIS) & api.albahjah.or.id
                                            └─ proxy ke 127.0.0.1:8080 (hris-api, systemd) ─► PostgreSQL 127.0.0.1:5432
```

Untuk web admin yang berbeda subdomain dengan API, keduanya harus satu domain induk agar cookie `SameSite=Strict` tetap terkirim (`COOKIE_DOMAIN=hris.local` di development bila memakai `admin.hris.local` + `api.hris.local`, atau `COOKIE_DOMAIN=albahjah.or.id` di production untuk `hris.albahjah.or.id` + `api.albahjah.or.id`), atau web admin dapat memanggil `/v1/` pada origin yang sama (`admin.hris.local`) karena `nginx-web.conf` juga meneruskan `/v1/` ke `127.0.0.1:8080`. Aplikasi Android tidak memakai cookie melainkan token `Bearer`, sehingga di development langsung terhubung ke `https://api.albahjah.net`.

## Alur Git

```
feat/...  ──PR──►  develop  ──bash deploy.sh di server──►  server development
```

Deploy dijalankan manual. **Pastikan CI branch `develop` hijau dulu**, karena skrip tidak memeriksanya.

## Penyiapan awal (sekali)

### 1. aaPanel
1. **App Store:** pasang Nginx dan PostgreSQL Manager (versi 16 ke atas). Catat kata sandi superuser `postgres`.
2. **Security / Firewall:** jangan buka port 5432 dan 8080 ke internet. Yang perlu terbuka hanya 80 dan 443, plus SSH.
3. **Website:** buat dua situs.
   - Development: `admin.hris.local` (situs statis web admin, root `/www/wwwroot/hris-web`) dan `api.albahjah.net` (API)
   - Production: `hris.albahjah.or.id` (situs statis web admin, root `/www/wwwroot/HRIS`) dan `api.albahjah.or.id` (API)
4. Pada domain publik (`api.albahjah.net`, `hris.albahjah.or.id`, `api.albahjah.or.id`), aktifkan **SSL Let's Encrypt** dan paksa HTTPS.

### 2. Alat build
Server butuh `git`, `rsync`, Go 1.27, dan Node.js 22. Cek dengan:

```bash
git --version; rsync --version | head -1; go version; node --version
```

Paket `apt` terlalu lama. Pasang Go dari [go.dev/dl](https://go.dev/dl/) ke `/usr/local/go` dan Node.js dari
[nodejs.org](https://nodejs.org/dist/) ke `/usr/local/node`, lalu cocokkan checksum-nya.

Build Next.js butuh memori sekitar 1,5 GB. Bila RAM server di bawah 3 GB, tambahkan swap dulu.

### 3. Ambil kode dan siapkan server (Development)
Sebagai root. Sesuaikan domain dan folder Node. Skrip menanyakan kata sandi `postgres`; kata sandi itu tidak disimpan.

```bash
git clone --branch develop https://github.com/rzarxx/HRIS.git /tmp/hris-setup
```

```bash
PSQL=/www/server/pgsql/bin/psql WEB_ORIGIN=http://admin.hris.local COOKIE_DOMAIN=hris.local NEXT_PUBLIC_API_URL=http://admin.hris.local WEB_ROOT=/www/wwwroot/hris-web NODE_BIN_DIR=/usr/local/node/bin bash /tmp/hris-setup/backend/deploy/setup-server.sh
```

```bash
rm -rf /tmp/hris-setup
```

Skrip membuat user `hris` dan `hris-build`, database `hris_dev`, kode di `/opt/hris/src`, `/opt/hris/.env`, `/opt/hris/deploy.env`,
dan memasang layanan `hris-api`. Bila repo dijadikan private, pakai deploy key baca-saja milik `hris-build`.

### 4. Nginx
Buat kedua situs sebagai PHP Project dengan PHP **Pure static**, tanpa database dan FTP. API dijalankan systemd, jadi jangan memakai Go Project.
Setelah kedua situs dibuat, pasang konfigurasinya. Cadangan otomatis dibuat dan dikembalikan bila `nginx -t` gagal.

```bash
API_SITE=api.albahjah.net WEB_SITE=admin.hris.local bash /opt/hris/src/backend/deploy/setup-nginx.sh
```

### 5. Deploy pertama dan data dummy

```bash
bash /opt/hris/src/backend/deploy/deploy.sh
```

```bash
cd /opt/hris && runuser -u hris -- ./hris-seed
```

### 6. Periksa

```bash
curl -i http://127.0.0.1:8080/readyz
```
API menolak akses langsung dari luar (D37), jadi perintah ini seharusnya menjawab `403 API_ACCESS_DENIED`:

```bash
curl -i https://api.albahjah.net/healthz
```

Postman ke server harus mengirim header `Origin` berisi alamat web admin. Pemeriksaan dari server lewat `127.0.0.1:8080` tidak terpengaruh.

Buka `http://admin.hris.local`, masuk dengan `sdm@pejuang.id`. Kata sandinya ada di server:

```bash
grep SEED_SDM_PASSWORD /opt/hris/.env
```

## Memperbarui versi
Setelah PR masuk ke `develop` dan CI hijau:

```bash
bash /opt/hris/src/backend/deploy/deploy.sh
```

Bila build gagal, API yang sedang berjalan tidak tersentuh. Migrasi hanya maju, jadi kembali ke versi lama tidak
membatalkan perubahan database.

## Perintah lain
Melihat log:

```bash
journalctl -u hris-api -n 100 --no-pager
```

Mengembalikan data dummy ke kondisi awal:

```bash
cd /opt/hris && runuser -u hris -- ./hris-seed --reset
```

Menyiapkan data demo: lima absen di Review Absen, enam absen hari ini di Monitor Absen, tiga izin kerja, dan dua permintaan perangkat. Perintah
ini aman diulang, dan mengembalikan data demo ke kondisi awal setelah latihan:

```bash
cd /opt/hris && runuser -u hris -- ./hris-seed --demo
```

## PostgreSQL aaPanel
Build PostgreSQL dari aaPanel tidak menyertakan ekstensi contrib seperti `pgcrypto`, dan PostGIS belum tentu tersedia.
PostGIS dibutuhkan mulai Fase 2, jadi cek dulu sebelum fase itu dimulai.

Bila migrasi gagal, database ditandai `dirty`. Setelah penyebabnya diperbaiki, hapus penandanya hanya bila tabel aplikasi belum
terbentuk, lalu deploy ulang.

## Tanpa domain
Bisa memakai `http://IP:PORT` untuk tiap situs, dengan `WEB_ORIGIN` dan `NEXT_PUBLIC_API_URL` berisi alamat itu dan `COOKIE_DOMAIN`
dikosongkan. Risikonya, kata sandi dan token terkirim tanpa enkripsi, jadi batasi aksesnya di firewall hanya untuk IP kantor
atau lewat VPN.

## Beralih ke Mode Produksi (`APP_ENV=production`)

Saat server siap digunakan untuk data Pejuang asli (HTTPS aktif di `hris.albahjah.or.id` dan `api.albahjah.or.id`):

1. Jalankan skrip pengalihan produksi sebagai `root`:
   ```bash
   WEB_ORIGIN=https://hris.albahjah.or.id COOKIE_DOMAIN=albahjah.or.id NEXT_PUBLIC_API_URL=https://api.albahjah.or.id bash /opt/hris/src/backend/deploy/setup-production.sh
   ```
   Skrip ini mengaktifkan `APP_ENV=production`, `MFA_REQUIRED=true`, `COOKIE_SECURE=true`, mematikan `DEV_ALLOW_UNVERIFIED_ATTENDANCE` dan `DEV_SIMULATE_ATTENDANCE`, membuat kunci passphrase backup `/etc/hris/backup.pass` (simpan salinan file ini di password manager terpisah!), mengaktifkan timer backup harian & uji restore mingguan, serta membangun biner `/opt/hris/hris-bootstrap`.

2. Deploy ulang agar web admin dan biner terbaru terpasang:
   ```bash
   bash /opt/hris/src/backend/deploy/deploy.sh
   ```

3. Buat akun SDM pertama (hanya bisa dijalankan satu kali saat belum ada akun ber-role `sdm`):
   ```bash
   cd /opt/hris && runuser -u hris -- ./hris-bootstrap --email sdm@albahjah.or.id --name "Admin SDM" --nip 1001
   ```
   Setelah login pertama, akun wajib mengganti kata sandi dan mengaktifkan MFA TOTP.

## Backup Terenkripsi & Verifikasi Restore Otomatis

`setup-production.sh` dan `deploy.sh` memasang dua timer systemd:
- `hris-backup.timer` (`01:30` setiap hari): menjalankan `backup.sh` untuk mencadangkan PostgreSQL (`pg_dump --format=custom`), `.env`, `deploy.env`, `.secrets`, dan `selfies/`, lalu mengenkripsinya dengan AES-256-CBC PBKDF2 (`/etc/hris/backup.pass`) ke `/var/backups/hris/`. Bila `OFFSITE_DEST` diisi di `/opt/hris/deploy.env` (mis. `rclone://gdrive:hris-backup` atau `user@host:/path`), arsip terenkripsi ikut dikirim ke lokasi terpisah.
- `hris-restore-verify.timer` (Ahad `03:10` setiap minggu): menjalankan `restore-verify.sh` untuk mendekripsi cadangan terbaru, memeriksa checksum `manifest.sha256` dan kunci, serta menguji `pg_restore` ke database sementara `hris_restore_verify_*`.

Menjalankan backup dan verifikasi restore secara manual:
```bash
bash /opt/hris/src/backend/deploy/backup.sh
bash /opt/hris/src/backend/deploy/restore-verify.sh
```

## Mencoba absen di server development

Pada `APP_ENV=development` sebelum layanan Google Play Integrity dihubungkan, tambahkan baris ini ke `/opt/hris/.env` lalu jalankan `systemctl restart hris-api`:

```bash
DEV_ALLOW_UNVERIFIED_ATTENDANCE=true
```

API menolak menyala bila baris ini dipakai di luar `APP_ENV=development`.

Simulasi presensi melewati Pejuang yang sudah punya perangkat. Daftarkan perangkat sebelum jam masuk,
karena bila baris simulasi hari ini sudah terisi, absen baru bisa dicoba keesokan harinya.

## Verifikasi wajah (D43)

Wajah dihitung API dari selfie dengan detektor YuNet dan pengenal SFace dari OpenCV Zoo, dijalankan
ONNX Runtime. Bagian ONNX Runtime memakai cgo, jadi disiapkan sekali dengan skrip berikut, sebagai root,
di Ubuntu atau Debian x86-64:

```bash
bash /opt/hris/src/backend/deploy/setup-face.sh
bash /opt/hris/src/backend/deploy/deploy.sh
```

Skrip mengunduh ONNX Runtime 1.31.0 ke `/opt/hris/onnxruntime` dan dua model (sekitar 39 MB) ke
`/opt/hris/face-models`, semuanya dengan checksum; membuat `/opt/hris/selfies`; menghapus model dlib
lama; lalu mengisi `FACE_MODELS_DIR`, `ONNXRUNTIME_LIB`, dan `SELFIE_DIR` di `/opt/hris/.env` serta
`FACE_ENGINE=onnx` di `deploy.env`. Butuh `gcc` (paket `build-essential`).

Versi ONNX Runtime harus sama dengan versi header `github.com/yalue/onnxruntime_go` di `go.mod`.
Saat pustaka itu diperbarui, perbarui juga `ORT_VERSION` dan checksum di `setup-face.sh`.

Periksa setelah deploy:

```bash
journalctl -u hris-api -n 20 --no-pager | grep -i wajah
```

Baris `verifikasi wajah aktif` dengan `model: sface-2021dec` berarti siap. Tanpa langkah ini API tetap
jalan, tetapi daftar wajah dan absen yang membawa selfie dijawab `FACE_SERVICE_UNAVAILABLE`.
`deploy.sh` berhenti sebelum mengganti biner bila `FACE_ENGINE` masih berisi nilai lama seperti `dlib`.

Template wajah dari model lain (dlib, D42) dianggap belum terdaftar, jadi Pejuang mendaftar ulang.

Selfie disimpan terenkripsi dengan kunci `FIELD_ENCRYPTION_KEY_PATH`. Folder selfie ikut dicadangkan
bersama kunci itu; tanpa kunci, selfie tidak bisa dibuka.

Selfie dihapus setelah 90 hari oleh `hris-purge-selfies.timer`, yang dipasang dan diaktifkan
`deploy.sh` (setiap hari pukul 02.17 waktu server, ditambah jeda acak sampai 10 menit; yang
terlewat saat server mati dijalankan begitu menyala). Hash selfie tetap disimpan untuk menolak foto
yang dikirim ulang. Service-nya dilewati bila `/opt/hris/selfies` belum ada. Periksa jadwal dan hasil
terakhir:

```bash
systemctl list-timers hris-purge-selfies.timer
journalctl -u hris-purge-selfies -n 20 --no-pager
```

Menjalankan sekali secara manual:

```bash
systemctl start hris-purge-selfies
```

## Catat alpha harian (mark_absent)

`hris-mark-absent.timer` dipasang dan diaktifkan `deploy.sh`. Setiap hari pukul 04.20 WIB (setelah
cutoff 04.00, ditambah jeda acak sampai 10 menit) job ini mencatat `absent` untuk Pejuang aktif
berakun yang punya shift, pada hari kerja shift yang bukan hari libur, tanpa presensi dan tanpa cuti
yang disetujui, sejak `join_date` dan sampai `exit_date`. Izin WFA/hybrid tidak membebaskan presensi.
Job memeriksa 7 hari tertutup terakhir, jadi hari yang terlewat saat server mati ikut tercatat;
baris yang sudah ada tidak pernah diubah. Cuti yang disetujui belakangan tetap menimpa alpha.

```bash
systemctl list-timers hris-mark-absent.timer
journalctl -u hris-mark-absent -n 20 --no-pager
```

Menjalankan sekali secara manual, mis. mengejar 30 hari:

```bash
cd /opt/hris && runuser -u hris -- ./hris-mark-absent -days 30
```

## Super admin

Super admin adalah role tertinggi: semua akses kecuali keuangan. Role diberikan lewat server, pada
akun Pejuang biasa yang sudah dibuat SDM:

```bash
cd /opt/hris && runuser -u hris -- ./hris-superadmin grant --email orang@contoh.id
```

Bila akun belum aktif, tautan aktivasi tampil di terminal. Untuk mencabut, pakai `revoke`. Bila
super admin lupa kata sandi, pakai `reset`, karena SDM tidak bisa mereset akun super admin.
