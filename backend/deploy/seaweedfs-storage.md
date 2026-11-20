# Penyimpanan berkas HRIS di SeaweedFS S3

API menyimpan selfie presensi, foto profil, lampiran pengumuman, rekap XLSX, dan template XLSX
di bucket privat melalui S3 API. Semua isi objek dienkripsi AES-GCM oleh API sebelum dikirim.
Nama objek tetap berupa kunci internal; tidak ada URL bucket yang diberikan ke browser atau HP.
Database PostgreSQL, kunci enkripsi, kunci JWT, model wajah, dan berkas konfigurasi tetap di
penyimpanan server yang sesuai dan harus masuk backup terpisah.

## Siapkan SeaweedFS

1. Jalankan gateway S3 SeaweedFS pada alamat loopback yang dapat diakses proses `hris-api`,
   misalnya `http://127.0.0.1:8333`. Jangan membuka gateway tanpa autentikasi ke internet.
   SeaweedFS perlu konfigurasi identity; gateway tanpa konfigurasi dapat menerima sembarang
   access key. Buat bucket privat khusus HRIS. Identity aplikasi hanya memerlukan aksi
   `Read:hris-private`, `Write:hris-private`, dan `List:hris-private`; aksi `Write` juga
   mengizinkan `DeleteObject`. Buat bucket dengan identity admin terpisah, lalu uji
   `PutObject`, `GetObject`, `DeleteObject`, `ListObjectsV2`, dan `HeadBucket` memakai
   identity aplikasi. Contoh konfigurasi S3 gateway (isi nilai kredensial secara privat):

   ```json
   {
     "identities": [{
       "name": "hris-app",
       "credentials": [{"accessKey": "<access-key>", "secretKey": "<secret-key>"}],
       "actions": ["Read:hris-private", "Write:hris-private", "List:hris-private"]
     }]
   }
   ```

   Untuk satu host dengan direktori data persisten yang sudah dimiliki user layanan SeaweedFS,
   contoh prosesnya:

   ```bash
   weed server -dir=/var/lib/seaweedfs -ip=127.0.0.1 -ip.bind=127.0.0.1 \
     -s3 -s3.port=8333 -s3.ip.bind=127.0.0.1 -s3.config=/etc/seaweedfs/s3.json
   ```

   Jalankan proses ini sebagai layanan yang terus hidup. Jangan menyimpan data SeaweedFS di
   direktori sementara. Konfigurasi S3 harus sudah memuat identity admin saat membuat bucket;
   setelah bucket tersedia, aplikasi cukup memakai identity terbatas di atas.
2. Untuk instalasi statis tanpa panel, simpan kredensial di environment privat API, bukan di Git. Atur:

   ```dotenv
   STORAGE_DRIVER=s3
   STORAGE_ENDPOINT=http://127.0.0.1:8333
   STORAGE_BUCKET=hris-private
   STORAGE_REGION=us-east-1
   STORAGE_ACCESS_KEY=<nilai privat>
   STORAGE_SECRET_KEY=<nilai privat>
   ```

   Untuk migrasi melalui panel di bawah, pertahankan `STORAGE_DRIVER=disk` dan isi kredensial
   hanya pada formulir Super Admin; jangan isi contoh environment di atas. `http` hanya diterima
   untuk loopback. Untuk host lain gunakan HTTPS. SDK memakai path-style
   URL agar bucket lokal tidak memerlukan DNS wildcard. `FIELD_ENCRYPTION_KEY_PATH` lama harus
   dipertahankan; mengganti kunci membuat objek lama tidak bisa dibuka.

## Migrasi dari SELFIE_DIR

Untuk server S3 yang sudah online, pertahankan `SELFIE_DIR` lama dan `STORAGE_DRIVER=disk`.
Jalankan backup yang sudah ada, lalu buka **Pengaturan → Penyimpanan S3** sebagai Super Admin.
Isi endpoint HTTPS, bucket, region, access key, dan secret key. Tombol **Uji koneksi dan aktifkan S3**
baru menyimpan konfigurasi setelah uji HeadBucket dan operasi tulis/baca/hapus berhasil. Secret key
disimpan terenkripsi dalam `app_settings` dan tidak dikirim kembali ke browser.

Setelah aktif, unggahan baru dan XLSX hasil generate menuju S3. Berkas lama tetap dibaca dari
`SELFIE_DIR` bila objeknya belum ada di S3. Tekan **Mulai / lanjutkan sinkronisasi**; panel
memanggil batch kecil berulang sampai selesai. Kursor tersimpan di database sehingga proses bisa
dilanjutkan setelah koneksi browser terputus. Setiap berkas didekripsi oleh API lalu dienkripsi
kembali di S3 dengan kunci lapangan yang sama; isi hasil baca dibandingkan. Objek berbeda pada
kunci yang sama menghentikan migrasi tanpa ditimpa. Salinan disk tidak dihapus otomatis.

Konfigurasi yang disimpan dari panel berlaku pada proses API lain dan CLI retensi pada operasi
berikutnya. Endpoint dan bucket aktif dikunci; rotasi access key/secret key pada bucket yang
sama boleh dilakukan. Untuk pindah bucket, siapkan migrasi antar-bucket secara terpisah. Jangan
hapus `SELFIE_DIR` sampai status migrasi selesai, satu unggah/unduh tiap jenis berkas teruji, dan
backup serta restore terverifikasi. Pastikan kunci `FIELD_ENCRYPTION_KEY_PATH` tidak berubah.

Untuk backup, konfigurasi `STORAGE_BACKUP_REMOTE` pada `/opt/hris/deploy.env` sebagai remote
`rclone` yang membaca bucket yang sama. Jalankan `backup.sh` dan `restore-verify.sh` setelah
S3 aktif. Skrip backup mendeteksi konfigurasi panel dari database dan ikut menyimpan salinan
disk selama migrasi. Bila remote ini belum ada, backup gagal tertutup; siapkan sebelum migrasi.

## Backup dan retensi

Saat S3 aktif dari panel atau `STORAGE_DRIVER=s3`, `backup.sh` mewajibkan `STORAGE_BACKUP_REMOTE`, yaitu remote rclone
yang membaca seluruh bucket, misalnya `seaweed:hris-private`. Skrip menyalin objek ke arsip
backup terenkripsi sebelum dan sesudah dump PostgreSQL, bersama secrets. `BACKUP_REMOTE_DEST` tetap dipakai
untuk mengirim arsip terenkripsi ke lokasi di luar server. Simpan juga `backup.pass` di vault
terpisah. Pastikan ruang sementara untuk menyalin seluruh isi bucket sebelum mengaktifkan
timer backup; kebutuhan ruang dan durasinya bertambah mengikuti ukuran bucket. SeaweedFS lokal
di server yang sama tidak menggantikan backup luar server.

Jalankan `restore-verify.sh` setelah backup pertama. Pada mode S3, pemeriksaan restore
memastikan setiap selfie, foto, dan lampiran yang masih dirujuk snapshot database ada dalam
arsip. Opsi `--archive-only` hanya memeriksa checksum arsip, tanpa pemeriksaan referensi
database. Uji pemulihan objek ke bucket staging secara berkala sebelum mengandalkan backup
untuk pemulihan produksi.

Timer `hris-purge-selfies` menghapus selfie setelah 90 hari sesuai database, dan objek di
`generated/` setelah 7 hari. XLSX yang baru dibuat tetap langsung dikirim lewat API setelah
berhasil disimpan. Objek hasil generate tidak memiliki URL publik dan tidak ditautkan ulang
oleh database. Workbook kata sandi awal dari impor Pejuang tetap hanya dikirim sekali dalam
respons impor dan tidak diarsipkan, agar kata sandi tidak tersimpan lebih lama. Berkas impor
yang diunggah untuk diproses juga tidak diarsipkan. Jika SeaweedFS atau bucket tidak tersedia,
`/readyz` gagal dan operasi berkas tidak dianggap berhasil.

## Uji integrasi di bucket sementara

Tes `TestSeaweedFSIntegration` memakai bucket uji khusus. Atur `HRIS_TEST_S3_ENDPOINT`,
`HRIS_TEST_S3_BUCKET`, `HRIS_TEST_S3_ACCESS_KEY`, dan `HRIS_TEST_S3_SECRET_KEY`, lalu jalankan
`go test -run '^TestSeaweedFSIntegration$' ./internal/platform/filestore` dari `backend/`.
Untuk membuat dan menghapus bucket uji secara otomatis, set `HRIS_TEST_S3_CREATE_BUCKET=1`
dan berikan identity admin terpisah melalui `HRIS_TEST_S3_ADMIN_ACCESS_KEY` serta
`HRIS_TEST_S3_ADMIN_SECRET_KEY`. Opsi ini juga menguji pembersihan `generated/`, sehingga
bucket harus khusus untuk tes. `HRIS_TEST_S3_CHECK_AUTH=1` memeriksa kredensial salah ditolak.

Referensi: [SeaweedFS S3 API](https://github.com/seaweedfs/seaweedfs/wiki/Amazon-S3-API),
[AWS SDK for Go v2 custom endpoints](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-endpoints.html).
