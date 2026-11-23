# Perintah mencoba SeaweedFS S3 di server produksi HRIS

Panduan ini untuk produksi aaPanel dengan kode di `/www/wwwroot/HRIS/src`, tautan
`/opt/hris/src` ke direktori itu, dan `hris-api` sebagai layanan systemd. PR S3
harus sudah masuk `main`. Jalankan **satu blok perintah pada satu waktu** sebagai
`root`; berhenti bila perintah gagal atau hasilnya berbeda dari yang dijelaskan.
Jangan tempel isi `.env`, `deploy.env`, `rclone.conf`, access key, secret key, atau
URL yang memuat kredensial ke chat.

SeaweedFS Anda sudah online: tidak perlu menjalankan `weed server` lagi. Siapkan
endpoint **HTTPS** tanpa path/bucket, bucket privat yang sudah dibuat, region,
dan identity aplikasi yang dapat membaca, menulis, menghapus, serta me-list
objek pada bucket itu. Gunakan bucket khusus HRIS. Database, kunci enkripsi,
model wajah, dan konfigurasi aplikasi tetap memerlukan backup di luar S3.

## 1. Periksa server sebelum mengubah apa pun

Pastikan shell adalah `root` (hasil `0`):

```bash
id -u
```

Pastikan tautan sumber mengarah ke direktori produksi yang diminta:

```bash
readlink -f /opt/hris/src
```

Hasil yang diharapkan: `/www/wwwroot/HRIS/src`. Jika berbeda, hentikan panduan
ini dan cocokkan layout server terlebih dahulu.

Periksa branch dan perubahan lokal dengan user pemilik checkout:

```bash
runuser -u hris-build -- env HOME=/opt/hris/build-home git -C /opt/hris/src status --short --branch
```

Harus menunjukkan branch `main` dan tidak ada daftar file berubah. Jangan
menjalankan deploy bila checkout kotor; skrip memakai `merge --ff-only`.

Periksa hanya pengaturan deploy yang aman ditampilkan:

```bash
grep -E '^(BRANCH|WEB_ROOT)=' /opt/hris/deploy.env
```

Harus `BRANCH=main` dan `WEB_ROOT=/www/wwwroot/HRIS`. Jika berbeda, hentikan
dan pastikan server ini memang produksi sebelum mengubahnya.

Periksa mode dan **path** penyimpanan tanpa menampilkan nilai kunci:

```bash
grep -E '^(APP_ENV|STORAGE_DRIVER|SELFIE_DIR|FIELD_ENCRYPTION_KEY_PATH)=' /opt/hris/.env
```

Pertahankan `SELFIE_DIR` dan `FIELD_ENCRYPTION_KEY_PATH` yang sudah dipakai.
Untuk konfigurasi lewat panel, `STORAGE_DRIVER` tetap `disk` atau belum diisi.
Jangan menambahkan kredensial S3 ke `.env`.

## 2. Cadangkan data lama sebelum deploy

Jalankan backup yang saat ini terpasang:

```bash
bash /opt/hris/src/backend/deploy/backup.sh
```

Lalu uji restore arsip terbaru ke database sementara:

```bash
bash /opt/hris/src/backend/deploy/restore-verify.sh
```

Lanjutkan hanya bila keduanya sukses. Pastikan arsip terenkripsi juga disalin
ke lokasi **di luar server HRIS**; `BACKUP_REMOTE_DEST` yang kosong hanya
menyimpan arsip lokal. Simpan `/etc/hris/backup.pass` di vault terpisah.

## 3. Deploy `main`

Skrip ini mengambil `main`, membangun API dan web, menjalankan migrasi maju,
me-restart API, dan memasang ulang timer. Jangan jalankan seed atau
`setup-production.sh` untuk pembaruan ini.

```bash
bash /opt/hris/src/backend/deploy/deploy.sh
```

Periksa commit terpasang. Merge S3 ke `main` pada 20 November 2026 adalah
`e1d8f29`; commit yang lebih baru juga boleh bila memuat fitur S3.

```bash
runuser -u hris-build -- env HOME=/opt/hris/build-home git -C /opt/hris/src log -1 --oneline
```

Pastikan API siap (HTTP `200`):

```bash
curl -i http://127.0.0.1:8080/readyz
```

Jika deploy atau readiness gagal, **jangan aktifkan S3**. Periksa status layanan
dan log setelah menyamarkan informasi sensitif:

```bash
systemctl status hris-api --no-pager
```

## 4. Siapkan pembaca bucket untuk backup

`backup.sh` menyalin isi bucket memakai `rclone` setelah S3 aktif dari panel.
Konfigurasi `rclone` perlu dapat dibaca layanan systemd: unit backup memakai
`ProtectHome=true`, sehingga konfigurasi bawaan di `/root/.config/rclone`
tidak dapat diakses dari unit tersebut.

Periksa apakah `rclone` sudah ada:

```bash
rclone version
```

Jika perintah tidak ditemukan di Ubuntu, jalankan dua perintah berikut lalu
ulang `rclone version`:

```bash
apt-get update
```

```bash
apt-get install -y rclone
```

Buat direktori konfigurasi privat:

```bash
install -d -m 700 /etc/rclone
```

Buat remote secara **interaktif**, tanpa menaruh secret di argumen shell.
Pilih nama remote `hris_s3`, tipe `s3`, provider `SeaweedFS` bila tersedia
(atau `Other`), kredensial manual, endpoint HTTPS yang sama dengan panel,
dan region yang sama. Biarkan path style aktif. Jika `BACKUP_REMOTE_DEST`
sebelumnya memakai remote rclone lain, buat juga remote itu di file konfigurasi
yang sama agar backup luar server tetap bekerja.

```bash
rclone --config /etc/rclone/rclone.conf config
```

Batasi izin file konfigurasi:

```bash
chmod 600 /etc/rclone/rclone.conf
```

Edit konfigurasi deploy secara lokal di server:

```bash
nano /opt/hris/deploy.env
```

Tambahkan atau sesuaikan dua baris berikut. Ganti `NAMA_BUCKET_HRIS` dengan
nama bucket yang benar, **tanpa tanda kurung atau spasi**. Jangan ubah baris
`BRANCH=main` dan pengaturan lain yang sudah benar.

```dotenv
RCLONE_CONFIG=/etc/rclone/rclone.conf
STORAGE_BACKUP_REMOTE=hris_s3:NAMA_BUCKET_HRIS
```

Periksa akses baca bucket. Bucket kosong boleh menghasilkan keluaran kosong,
tetapi exit code harus sukses. Isi nama bucket saat diminta:

```bash
read -r -p 'Nama bucket HRIS: ' HRIS_S3_BUCKET
```

```bash
rclone --config /etc/rclone/rclone.conf lsf "hris_s3:${HRIS_S3_BUCKET}" --max-depth 1
```

Sebelum backup S3, periksa ruang sementara: skrip menyalin seluruh bucket dan
salinan disk lama ke `/tmp` sebelum membuat arsip terenkripsi di `/opt/hris`.

```bash
df -h /tmp /opt/hris
```

## 5. Aktifkan dari panel Super Admin

1. Masuk ke `https://hris.albahjah.or.id/pengaturan` sebagai Super Admin.
2. Pada **Penyimpanan S3 (SeaweedFS)**, isi endpoint HTTPS, bucket, region,
   access key, dan secret key. Isi secret hanya pada panel. Tekan **Uji koneksi
   dan aktifkan S3**. Server menguji bucket serta operasi tulis, baca, dan
   hapus sebelum menyimpan pengaturan.
3. Periksa bahwa badge menampilkan **Aktif**. Mulai **sinkronisasi** berkas
   lama dan biarkan berjalan sampai panel menampilkan **Selesai**. Bila
   terhenti, tombol yang sama dapat melanjutkan dari kursor tersimpan.
4. Coba unggah dan baca kembali foto profil, lampiran pengumuman, dan selfie
   pada alur uji yang sesuai. Periksa hanya data
   uji yang aman dipakai di produksi.

Setelah aktif, cek readiness lagi (harus HTTP `200`):

```bash
curl -i http://127.0.0.1:8080/readyz
```

Jangan hapus `SELFIE_DIR` lama. API tetap memakainya sebagai fallback sampai
migrasi dan pemulihan data benar-benar terverifikasi.

## 6. Uji backup S3 melalui layanan yang sebenarnya

Jalankan unit backup agar pengaturan `rclone` diuji dalam lingkungan systemd
yang sama dengan timer. Proses dapat lama sesuai ukuran bucket.

```bash
systemctl start hris-backup.service
```

Pastikan hasil layanan `success`:

```bash
systemctl show -p Result hris-backup.service
```

Jalankan uji restore arsip terbaru, termasuk pemeriksaan referensi berkas
database terhadap arsip S3 **atau** salinan disk:

```bash
systemctl start hris-restore-verify.service
```

Pastikan hasil layanan `success`:

```bash
systemctl show -p Result hris-restore-verify.service
```

Periksa timer tetap aktif:

```bash
systemctl list-timers hris-backup.timer hris-restore-verify.timer hris-purge-selfies.timer --no-pager
```

`restore-verify.sh` masih menerima berkas yang hanya ada dalam salinan disk.
Status **Selesai** di panel, uji baca per jenis berkas, dan backup/restore
yang sukses semuanya diperlukan sebelum mempertimbangkan pensiun disk lama.
Untuk saat ini, **biarkan salinan disk tetap ada**.

Referensi: [panduan S3 HRIS](seaweedfs-storage.md),
[rclone S3](https://rclone.org/s3/),
[rclone config](https://rclone.org/docs/#config-string), dan
[systemd ProtectHome](https://www.freedesktop.org/software/systemd/man/257/systemd.exec.html).
