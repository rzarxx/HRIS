# Catatan Perubahan

Perubahan penting yang terlihat oleh Pejuang, SDM, atau pengelola server. Keputusan desain lengkap ada di
`MASTER.md` §14 (nomor D), kontrak API di `backend/api/openapi.yaml`. Format mengikuti
[Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/).

## Belum dirilis

### Ditambahkan
- **Profil tersinkron.** Pejuang bisa mengubah sendiri foto profil, nomor WA, dan email (dari aplikasi: Profil →
  Data pribadi; dari web: Pengaturan → Akun saya), serta data anak (aplikasi: Profil → Data anak). Perubahan langsung
  tampil di web dan aplikasi karena memakai data yang sama. Ganti email wajib kata sandi saat ini karena email juga
  dipakai untuk masuk. Foto disimpan terenkripsi di server, dipotong persegi 512 px, dan metadata EXIF-nya (termasuk
  lokasi) dibuang. Foto tampil di header web serta Beranda dan Profil aplikasi. Endpoint `PATCH /v1/me`,
  `GET`/`PUT /v1/me/children`, `PUT`/`DELETE /v1/me/photo`, `GET /v1/employees/{id}/photo`; migrasi 17.
- **Impor massal Pejuang dari xlsx (D53).** Master Data → Pejuang → **Impor xlsx**: unduh template, isi, lalu unggah.
  Template berisi daftar pilihan (cabang, divisi, subdiv, jenis kelamin, status pernikahan, status dan pendidikan
  anak), sheet Struktur, dan sheet Petunjuk. Sistem memeriksa semua baris lebih dulu dan baru menyimpan bila semuanya
  sesuai, dalam satu transaksi (maks 1.000 baris, 5 MB). NIPY yang sudah terdaftar memperbarui datanya tanpa mengubah
  kata sandi; baris lain menjadi Pejuang baru dengan kata sandi awal yang daftarnya bisa diunduh sebagai xlsx.
  Endpoint `GET /v1/admin/employees/import-template` dan `POST /v1/admin/employees/import`, audit `employee.imported`.

### Diubah
- Nama aplikasi dan web menjadi **HRIS Al Bahjah** (tanpa "Yayasan"). Artefak AAB rilis bernama `hris-albahjah`.
- Package aplikasi Android menjadi `id.albahjah.hris` (sebelumnya `id.pejuang.hris`). Di Google Play, package yang
  berbeda berarti aplikasi baru, sehingga aplikasi lama tidak diperbarui otomatis ke versi ini.
- Logo web dan aplikasi diganti logo Lembaga Pengembangan Dakwah Al-Bahjah, termasuk ikon tab browser dan ikon
  aplikasi Android. Halaman masuk web menampilkan logo lebih besar.
- Bahasa pesan di web, aplikasi, dan API diperhalus: kata seperti "ditolak", "gagal", "salah", dan "tidak valid"
  diganti kalimat yang lebih santun dan memberi arahan, misalnya "Presensi belum dapat dicatat. Silakan hubungi SDM."
  Status pengajuan "Ditolak" kini tampil "Tidak disetujui", dan upaya presensi yang tidak tercatat tampil "Tidak diterima".
- Kata sandi awal di-hash dengan argon2id parameter ringan agar impor ratusan Pejuang selesai dalam satu request
  (sekitar 4 detik untuk 1.000 baris). Kata sandi pilihan sendiri tetap memakai parameter penuh.

### Diperbaiki
- Batas 72 jam kata sandi awal kini benar-benar tersimpan untuk Pejuang baru yang ditambahkan lewat form. Sebelumnya
  hanya tersimpan saat reset kata sandi, sehingga kata sandi awal Pejuang baru tidak kedaluwarsa.

### Perhatian saat memperbarui
- Bila server memakai `PLAY_INTEGRITY_PACKAGE` atau `KEY_ATTESTATION_PACKAGE`, ganti nilainya menjadi
  `id.albahjah.hris` bersamaan dengan rilis aplikasi baru. Daftarkan juga package baru di Play Console dan Google Cloud
  (Play Integrity).
- Pejuang yang ditambahkan sejak v0.10.0 dan belum mengganti kata sandi awal tidak punya batas 72 jam. Bila perlu,
  SDM dapat menekan Reset kata sandi untuk menerbitkan kata sandi awal baru yang berlaku 72 jam.

## v0.10.0 (16 November 2026)

### Ditambahkan
- **Cabang (D53).** Master Data → Cabang untuk cabang yayasan dengan divisi, subdiv, dan atasan sendiri. Divisi
  wajib memilih cabang dan kodenya unik per cabang. Daftar Pejuang dan divisi bisa disaring per cabang
  (`branch_id`). Semua divisi lama masuk ke cabang **Pusat**.
- **Data Pejuang mengikuti formulir yayasan.** Jenis kelamin, status pernikahan, memiliki anak, jumlah anak, dan
  data tiap anak (nama, jenis kelamin, tempat dan tanggal lahir, anak ke-, status kandung/angkat/tiri, jenjang
  pendidikan, nama sekolah). Data anak hanya untuk SDM, super admin, dan Pejuang sendiri.
- **Login dengan NIPY, email, atau nomor WA** di web dan aplikasi. Email tidak lagi wajib.
- **Penyelesaian Fase 1 (Fondasi Produksi):**
  - CLI `hris-bootstrap` (`backend/cmd/bootstrap`) untuk membuat akun SDM pertama secara aman di `APP_ENV=production` tanpa data dummy (`must_change_password = true`, menolak berjalan bila akun SDM sudah ada).
  - Otomasi backup harian terenkripsi AES-256-CBC (`backend/deploy/backup.sh`, `hris-backup.timer`) dan verifikasi restore otomatis mingguan ke database sementara (`backend/deploy/restore-verify.sh`, `hris-restore-verify.timer`).
  - Skrip pengalihan konfigurasi produksi (`backend/deploy/setup-production.sh`).
- **Penyelesaian Fase 2 (Verifikasi Perangkat Berjenjang & Bukti Kunci Instalasi, D24):**
  - Pengikatan kunci publik instalasi ECDSA P-256 (`devices.public_key`) saat login/registrasi perangkat dan verifikasi tanda tangan `device_proof` (`HRIS-DEVICE-PROOF-V1\n<request_hash>`) pada setiap presensi dan pendaftaran wajah.
  - Dukungan penuh jalur verifikasi perangkat (`devices.integrity_tier`: `play`, `attestation`, `manual`, `kiosk`), verifikasi rantai sertifikat X.509 Android Key Attestation (`internal/platform/integrity/attestation.go`), endpoint `POST /v1/admin/devices/{id}/tier`, audit `device.integrity_tier_changed`, dan modal pengaturan jalur verifikasi di halaman web admin `/perangkat`.
  - Suite tes skenario serangan presensi (`internal/attendance/attack_scenarios_test.go`) yang menguji penolakan fake GPS, root, emulator, foto diam, replay selfie, ketidakcocokan `request_hash`/kunci perangkat, Key Attestation, jalur manual (`+30` → `review`), dan kiosk kantor di luar geofence.
  - Konfigurasi ambang wajah, gerakan liveness, dan skor risiko lewat `.env` (`FACE_THRESHOLD_PASS`, `FACE_THRESHOLD_REVIEW`, `FACE_ENROLL_MIN_SIMILARITY`, `FACE_ENROLL_REQUIRES_GEOFENCE`, `FACE_STRAIGHT_TURN_MAX`, `FACE_GESTURE_TURN_MIN`, `FACE_GESTURE_TURN_DELTA`, `FACE_GESTURE_MIN_SIMILARITY`, `RISK_REVIEW_MIN`, `RISK_REJECT_MIN`), penanganan `MEETS_BASIC_INTEGRITY` sebagai `integrity_basic_only` (+30 → `review`), pembangkitan rantai sertifikat Android Key Attestation di aplikasi mobile (`DeviceIdentity.attestationChainBase64`), dan CLI kalibrasi otomatis `hris-calibrate` (`backend/cmd/calibrate`).
- **Liveness dibuktikan server (D51).** Aplikasi mengirim foto saat kepala menoleh (`gesture`) bersama selfie
  lurus. Server memastikan dari lima titik wajah bahwa selfie lurus, foto gerakan benar-benar menoleh, dan
  keduanya orang yang sama; bila tidak, `LIVENESS_FAILED`. Berlaku untuk presensi masuk, pulang, dan daftar
  wajah. Hash foto gerakan ikut diikat ke `request_hash`.
- **Sinyal `liveness_retry` (+25) dihitung server** dari dua upaya wajah yang ditolak dalam 30 menit, bukan dari
  laporan aplikasi.
- **Pemeriksaan HP sebelum presensi.** Aplikasi mendeteksi root, emulator, dan aplikasi pemalsu lokasi sebelum
  meminta challenge, menampilkan pesan yang jelas, dan mengirim hasilnya sebagai `signals`.
- Wajah harus pas di oval sebelum liveness dan foto (PR #90): pesan arahan, oval hijau dengan cincin kemajuan,
  pemeriksaan ulang foto hasil jepretan, dan server menolak wajah utama yang terlalu kecil atau tidak di tengah.
- **Job `mark_absent` (MASTER.md §11.3).** Setiap hari pukul 04.20 WIB, setelah cutoff, server mencatat alpha
  untuk Pejuang aktif berakun aktif pada hari kerja shift yang bukan hari libur, tanpa presensi dan tanpa cuti
  yang disetujui, sejak `join_date` sampai `exit_date`. Job mengejar 7 hari terakhir yang sudah tertutup dan tidak
  mengubah baris yang sudah ada; cuti yang disetujui belakangan tetap menimpa alpha. Kolom Alpha di rekap dan
  potongan alpha di pratinjau ubar kini terisi dari data nyata.
- **Rekap presensi lebih rinci.** Kolom baru `review` (presensi yang menunggu Review Presensi), cuti per jenis
  (`leave_by_type`) beserta ringkasan bersaldo, tanpa saldo, tanpa pengajuan, dan tidak dibayar, total jam kerja
  (`worked_minutes`), dan jumlah hari lupa presensi pulang (`missed_clock_out`). Semuanya tampil di halaman Rekap
  presensi dan ikut diekspor ke CSV, termasuk satu kolom per jenis cuti.

### Diubah
- **Akun baru langsung aktif dengan kata sandi awal** `@` + nama depan + tahun dibuat (contoh `@Siti2026`) yang
  wajib diganti saat login pertama (D53). Kata sandi awal hanya berlaku 72 jam; lewat dari itu login ditolak
  `AUTH_INITIAL_PASSWORD_EXPIRED` dan SDM perlu mereset ulang. Setelah masuk, web dan aplikasi langsung menampilkan
  popup ganti kata sandi. Tautan aktivasi, halaman `/aktivasi`, dan endpoint
  `/v1/auth/activate` dihapus. Reset kata sandi oleh SDM dan `hris-superadmin reset` mengembalikan akun ke kata
  sandi awal.
- Nomor WA disimpan dalam bentuk `62…` dan unik; masukan `08…`, `+62…`, atau dengan spasi dan tanda minus
  diterima.
- Aplikasi Android menargetkan API 36 (Android 16), syarat Google Play sejak 31 Agustus 2026; Android Gradle
  Plugin dinaikkan ke 8.13.2 (Gradle 9.2.1). Kotlin naik ke 2.3.21 dan Paparazzi ke 2.0.0-alpha04 (layoutlib API 36),
  sehingga Gradle di CI dan lokal butuh JDK 21.
- Workflow rilis (`release.yml`) kini membangun Android App Bundle bertanda tangan kunci unggah sebagai artefak
  workflow untuk diunggah ke Play Console. APK tidak lagi diterbitkan publik di GitHub Releases; rilis GitHub hanya
  berisi catatan perubahan.
- Web admin bernama **HRIS Yayasan Al Bahjah** dengan logo PUTI di sidebar, halaman masuk, dan ikon tab.
- Web admin memasang Content-Security-Policy (MASTER.md §5.8) lewat `<meta>` dengan hash skrip inline.
- Distribusi dan pembaruan aplikasi Android beralih dari GitHub Releases ke Google Play Store (build Android App
  Bundle `.aab` dari Android Studio, izin `REQUEST_INSTALL_PACKAGES` dan modul pembaruan GitHub dihapus, D52);
  domain produksi ditetapkan ke `hris.albahjah.or.id` (`/www/wwwroot/HRIS`) dan `api.albahjah.or.id`.
- Challenge hanya meminta `turn_left`/`turn_right`; `blink` dan `smile` tidak diminta lagi karena tidak bisa
  dibuktikan server.
- Sampel daftar wajah harus menghadap lurus; sampel yang menoleh ditolak `FACE_NOT_DETECTED`.
- Serialisasi `request_hash` presensi dan pendaftaran wajah menambahkan `gesture_sha256` di akhir (contoh baru di
  `MASTER.md` §5.4).
- Hari kerja di rekap kini menghitung presensi yang menunggu Review Presensi. Sebelumnya presensi `review` tidak
  masuk ke kolom mana pun.

### Diperbaiki
- `hris-superadmin grant` dan `reset` menampilkan kata sandi awal hanya di terminal interaktif, bukan stdout, sehingga
  tidak ikut tercatat bila keluaran dialihkan ke log (temuan CodeQL). Tanpa terminal, perintah menolak berjalan sebelum
  mengubah apa pun.
- `backup.sh` dan `restore-verify.sh` kini menemukan `pg_dump`, `pg_restore`, dan `psql` dari PostgreSQL aaPanel
  (`/www/server/pgsql/bin`). Sebelumnya backup harian di server aaPanel gagal dengan "Perintah wajib tidak ditemukan: pg_dump".
- `setup-production.sh` kini juga mengisi `CORS_ALLOWED_ORIGINS`. Sebelumnya hanya `WEB_ORIGIN` yang diganti,
  sehingga web admin di `hris.albahjah.or.id` ditolak API setelah pindah ke produksi.
- Tes integrasi cuti tidak lagi gagal saat tanggal ujinya sama dengan hari ini.

### Perhatian saat memperbarui
- **Migrasi 000016** membuat cabang Pusat, menormalkan nomor WA ke `62…`, dan **mengosongkan nomor yang tidak
  valid atau dipakai lebih dari satu Pejuang** (nomor yang tersisa harus unik). Periksa nomor WA di Master Data
  setelah migrasi.
- Akun lama yang belum pernah aktivasi tetap nonaktif. SDM menekan Reset kata sandi untuk menerbitkan kata sandi
  awal; akun super admin lewat `hris-superadmin grant`.
- Aplikasi Android lama masih bisa login dengan email karena server tetap menerima field `email`; login dengan
  NIPY atau nomor WA butuh aplikasi baru.
- **Server dan aplikasi harus diperbarui bersamaan.** Server baru menolak aplikasi lama yang tidak mengirim foto
  gerakan (`VALIDATION_FAILED`, field `gesture`), dan server lama menolak payload aplikasi baru.
- Ambang toleh kepala (selfie lurus ≤ 0.20, foto gerakan ≥ 0.18, selisih ≥ 0.10, kemiripan ≥ 0.30) adalah nilai
  awal dari geometri wajah. Kalibrasi dengan foto dari HP Android 10 sungguhan sebelum pilot.
- Liveness ini menolak foto diam (cetak atau layar), tetapi video wajah yang diputar ulang masih bisa lolos.
  Verifikasi perangkat di server (jalur B, Key Attestation) masih dikerjakan terpisah.
- `deploy.sh` membangun `hris-mark-absent` dan memasang `hris-mark-absent.timer`. Saat pertama aktif, job mencatat
  alpha untuk 7 hari kerja terakhir; pastikan presensi lewat aplikasi memang sudah dipakai. Untuk mengejar lebih
  jauh jalankan `./hris-mark-absent -days 30` (maks 62).
- Rekap per periode cutoff ubar dan aturan upah (D29/D31) belum termasuk; menyusul di Fase 5.
