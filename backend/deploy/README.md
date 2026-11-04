# Deploy ke server development (aaPanel + Git)

Server development memakai **data dummy**. Jangan isi data karyawan asli: mode ini mematikan MFA dan mengizinkan seed.
Nama menu dan lokasi folder aaPanel bisa sedikit berbeda antar versi.

```
Browser ─ HTTPS ─► Nginx aaPanel ─┬─ admin.<domain> : file statis /www/wwwroot/hris-web
                                  └─ api.<domain>   : proxy ke 127.0.0.1:8080 (hris-api, systemd)
                                                       └─ PostgreSQL 127.0.0.1:5432
```

Dua subdomain itu harus satu domain induk agar cookie `SameSite=Strict` tetap terkirim. `COOKIE_DOMAIN` diisi domain induk
agar web admin bisa membaca `hris_csrf`; tanpa itu semua aksi simpan ditolak `CSRF_INVALID`.

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
   - `admin.<domain>`, situs statis dengan root `/www/wwwroot/hris-web`
   - `api.<domain>`
4. Di tiap situs, aktifkan **SSL Let's Encrypt** dan paksa HTTPS.

### 2. Alat build
Server butuh `git`, `rsync`, Go 1.27, dan Node.js 22. Cek dengan:

```bash
git --version; rsync --version | head -1; go version; node --version
```

Paket `apt` terlalu lama. Pasang Go dari [go.dev/dl](https://go.dev/dl/) ke `/usr/local/go` dan Node.js dari
[nodejs.org](https://nodejs.org/dist/) ke `/usr/local/node`, lalu cocokkan checksum-nya.

Build Next.js butuh memori sekitar 1,5 GB. Bila RAM server di bawah 3 GB, tambahkan swap dulu.

### 3. Ambil kode dan siapkan server
Sebagai root. Sesuaikan domain dan folder Node. Skrip menanyakan kata sandi `postgres`; kata sandi itu tidak disimpan.

```bash
git clone --branch develop https://github.com/rzarxx/HRIS.git /tmp/hris-setup
```

```bash
PSQL=/www/server/pgsql/bin/psql WEB_ORIGIN=https://admin.domain.id COOKIE_DOMAIN=domain.id NEXT_PUBLIC_API_URL=https://api.domain.id NODE_BIN_DIR=/usr/local/node/bin bash /tmp/hris-setup/backend/deploy/setup-server.sh
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
API_SITE=api.domain.id WEB_SITE=admin.domain.id bash /opt/hris/src/backend/deploy/setup-nginx.sh
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
```bash
curl -i https://api.domain.id/healthz
```

Buka `https://admin.domain.id`, masuk dengan `sdm@pejuang.id`. Kata sandinya ada di server:

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

## Sebelum dipakai data asli
Mode ini khusus development. Untuk data asli wajib `APP_ENV=production`, MFA aktif, HTTPS, kunci dari secret manager, backup teruji,
dan cara membuat akun SDM pertama (seed ditolak di luar development, dan bootstrap akun belum ada).

## Mencoba absen sebelum verifikasi selesai

Absen ditolak bawaan, karena verifikasi perangkat dan wajah belum dibuat. Hanya untuk uji di server
development, tambahkan baris ini ke `/opt/hris/.env` lalu jalankan `systemctl restart hris-api`:

```bash
DEV_ALLOW_UNVERIFIED_ATTENDANCE=true
```

API menolak menyala bila baris ini dipakai di luar `APP_ENV=development`.

Simulasi presensi melewati Pejuang yang sudah punya perangkat. Daftarkan perangkat sebelum jam masuk,
karena bila baris simulasi hari ini sudah terisi, absen baru bisa dicoba keesokan harinya.

## Super admin

Super admin adalah role tertinggi: semua akses kecuali keuangan. Role diberikan lewat server, pada
akun Pejuang biasa yang sudah dibuat SDM:

```bash
cd /opt/hris && runuser -u hris -- ./hris-superadmin grant --email orang@contoh.id
```

Bila akun belum aktif, tautan aktivasi tampil di terminal. Untuk mencabut, pakai `revoke`. Bila
super admin lupa kata sandi, pakai `reset`, karena SDM tidak bisa mereset akun super admin.
