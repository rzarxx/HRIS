# Catatan Perubahan

Perubahan penting yang terlihat oleh Pejuang, SDM, atau pengelola server. Keputusan desain lengkap ada di
`MASTER.md` §14 (nomor D), kontrak API di `backend/api/openapi.yaml`. Format mengikuti
[Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/).

## Belum dirilis

### Ditambahkan
- **Pulang otomatis untuk yang lupa absen pulang (D69).** Job harian kini menutup presensi tanpa jam pulang dengan jam
  akhir shift (atau jam masuk bila tanpa shift). Status tidak berubah dan periode ubar yang sudah diajukan dilewati.
  - Review Presensi punya bagian **Pulang otomatis**: SDM memilih **Sesuai**, **Ubah jam pulang**, atau **Tidak hadir**.
    Setiap keputusan dicatat `attendance.adjusted`; presensi sendiri dicek SDM lain.
  - Riwayat presensi di aplikasi memberi label "Pulang otomatis".
  - Endpoint `GET /v1/admin/attendance/auto-clock-outs` dan `POST /v1/admin/attendance/{id}/adjust`.
- **Reset A2F oleh super admin (D71).** Tombol **Reset A2F** di Master → Pejuang untuk akun SDM atau super admin lain
  yang kehilangan authenticator. Sesinya dicabut dan ia memindai kode QR baru saat masuk berikutnya. Untuk akun sendiri:
  `hris-superadmin reset-mfa --email ...` di server. Endpoint `DELETE /v1/admin/users/{user_id}/mfa`, audit
  `user.mfa_reset`.
- **Tab Slip di aplikasi (D68).** Tab Slip kini berisi slip ubar per periode, terbaru di atas.
  - Rincian slip: ubar bersih, ringkasan presensi, pendapatan, dan potongan. Tetap bisa dibaca saat offline.
  - **Lihat PDF** di dalam aplikasi, **Unduh** ke folder Unduhan (Android 8 dan 9 memilih lokasi simpan), dan
    **Bagikan** ke aplikasi lain. Layar slip tidak bisa di-screenshot.
  - Slip baru memunculkan banner "Slip ubar … sudah terbit" di Beranda dan lencana di tab Slip sampai dibuka.
- **Slip ubar terbit untuk Pejuang (D68).** Saat periode difinalkan:
  - Slip tersedia lewat `/v1/payslips` dan bisa diunduh sebagai PDF (kop yayasan, ringkasan presensi, pendapatan,
    potongan, ubar bersih dan terbilang, nomor dokumen).
  - Pejuang mendapat notifikasi di aplikasi (`/v1/notifications`) dan email pemberitahuan tanpa angka ubar bila SMTP
    aktif.
  - SDM bisa mengunduh PDF tiap Pejuang dari rincian periode; sebelum final bertanda DRAF.
  - Komponen yang tidak tampil di slip tetap dihitung tetapi tidak dikirim ke Pejuang.
- **Periode ubar dengan persetujuan dua SDM (D67).** Menu Payroll → **Periode Ubar** menggantikan Pratinjau.
  - SDM membuat periode per bulan (rentang dari tanggal cutoff), menekan **Hitung ubar**, mengisi lembur atau isian
    manual per Pejuang, lalu **Ajukan**.
  - Pengajuan ditolak bila masih ada presensi menunggu review atau lupa absen pulang.
  - SDM lain menyetujui (**Finalkan**) atau **Tolak** dengan alasan; yang menghitung atau mengajukan tidak bisa
    menyetujui sendiri. Setelah transfer, tandai **Sudah dibayar**.
  - Sejak diajukan, presensi dan persetujuan cuti di dalam periode itu terkunci.
  - Rekap presensi dan ekspor Excel mengikuti tanggal cutoff dari setelan periode.
  - Pratinjau ubar dengan rumus sementara dan endpoint `/v1/admin/reports/payroll-preview` dihapus.
- **Komponen ubar diatur SDM di panel (D66).** Menu Payroll kini berisi **Komponen Ubar** dan **Pratinjau**.
  - SDM menambah tunjangan dan potongan dengan salah satu cara hitung: nominal tetap, nominal per Pejuang, per hari
    hadir, per kali atau per menit telat, per hari alpha, per hari cuti tidak dibayar, persen dari ubar pokok, persen
    dari total pendapatan (mis. PPh 21), per satuan (mis. jam lembur), atau isian manual per periode.
  - Komponen yang salah cukup diubah, yang tidak dipakai cukup dinonaktifkan; tidak perlu mengubah kode.
  - **Setelan periode**: tanggal cutoff (bawaan 25) dan ubar bersih tidak pernah minus.
  - Master → Pejuang: tombol **Komponen ubar** untuk mengisi nominal per Pejuang, mis. tunjangan jabatan.
  - Data contoh (devsim) memuat contoh komponen dalam keadaan nonaktif.
  - Endpoint `/v1/admin/payroll/components`, `/v1/admin/payroll/settings`, dan
    `/v1/admin/employees/{id}/payroll-components`, khusus SDM, audit `payroll.rules_changed`.
- **Ingat NIPY dan kata sandi (D64).** Menggantikan kunci aplikasi dari 0.14.0.
  - Layar masuk mengingat NIPY dan kata sandi (centang **Ingat NIPY dan kata sandi**, bawaan aktif), lalu keduanya
    langsung terisi saat masuk lagi, termasuk setelah keluar. Kata sandi disimpan terenkripsi di HP dan tidak bisa
    ditampilkan.
  - **Gunakan akun lain** menghapus data masuk yang diingat. Bila kata sandi diganti dari web, aplikasi meminta kata
    sandi terbaru; ganti kata sandi dari aplikasi ikut memperbarui simpanan.
  - Masuk cepat dengan sidik jari, wajah, atau PIN HP tidak jadi dipakai (D65).
- **Pemberitahuan versi baru aplikasi (D63).** Super admin mengisi versi terbaru, versi minimum, tautan, dan catatan
  rilis di Pengaturan → **Versi aplikasi Android**.
  - Aplikasi yang lebih lama menampilkan banner **Versi baru tersedia** dengan tombol **Perbarui**. Bila terpasang dari
    Play Store, pembaruan memakai layar bawaan Play; selain itu tautan pembaruan dibuka.
  - Aplikasi di bawah versi minimum menampilkan layar **Perbarui aplikasi** dan server menolak permintaannya
    (426 `APP_UPDATE_REQUIRED`). Web tidak terpengaruh.
  - Profil → **Versi aplikasi** kini memeriksa pembaruan.
  - Endpoint `GET /v1/app/version` dan `GET`/`PUT /v1/admin/settings/app-version`, audit `settings.changed`.
- **Aplikasi tetap bisa dipakai saat internet putus (D62).** Aplikasi tidak lagi keluar sendiri saat sinyal hilang
  atau server sedang sibuk; sesi hanya berakhir bila server menolaknya.
  - Profil, riwayat presensi, pengumuman beserta lampiran yang pernah dibuka, pengajuan, dan persetujuan disimpan
    terenkripsi di HP, langsung tampil saat aplikasi dibuka, lalu diperbarui dari server.
  - Saat offline muncul banner **Mode offline** dengan waktu data terakhir dan tombol **Coba lagi**. Presensi dan
    pengajuan tetap memerlukan internet.
  - Data di HP dihapus saat keluar.
- **Ingat NIPY/email dan kunci sidik jari (D62).** Layar masuk punya centang **Ingat NIPY/email**; kata sandi tidak
  disimpan aplikasi. Setelah masuk, aplikasi menawarkan kunci dengan sidik jari, wajah, atau PIN HP (Android 10 ke
  atas): aplikasi tetap masuk, hanya dibuka dengan sidik jari saat dibuka dan setelah 5 menit di latar. Bisa diatur di
  Profil → **Kunci sidik jari / PIN**.
- **Shift ganda dan setengah hari (D61).** Satu Pejuang bisa punya beberapa shift dengan hari kerja berbeda, misalnya
  Senin–Jumat reguler dan Sabtu setengah hari.
  - Presensi masuk, alpha otomatis, dan monitor memilih shift yang hari kerjanya cocok.
  - Shift punya jenis **Reguler** atau **Setengah hari**; hari dari shift setengah hari dihitung 0,5 di kolom hari kerja
    rekap. Potongan payroll tidak berubah.
  - Form Pejuang memakai pilihan banyak shift dengan peringatan bila hari kerjanya bertabrakan; shift yang dipilih
    pertama menjadi shift utama.
  - Impor: kolom Shift menerima beberapa nama dipisah koma, dan sheet Shift mendapat kolom Jenis.
  - API Pejuang menerima `shift_ids` dan memuat `shifts`; `default_shift_id` tetap berlaku sebagai shift utama. Migrasi 22.
- **Super admin mengatur akun SDM dan A2F per akun (D60).** Di Master Data → Pejuang, super admin bisa:
  - menjadikan Pejuang sebagai SDM atau mengembalikannya ke Pejuang; sesi orang itu dicabut agar peran baru langsung
    berlaku;
  - menjadikan A2F wajib atau opsional untuk akun SDM tertentu (bawaan tetap wajib). Super admin selalu wajib A2F, dan
    A2F yang sudah aktif tetap diminta saat masuk.

  Kepala unit tetap mengikuti Struktur Organisasi dan peran super admin tetap hanya lewat perintah server. Endpoint
  `PUT /v1/admin/users/{user_id}/role` dan `/mfa-policy`, permission baru `account:manage`, audit
  `user.mfa_policy_changed`; migrasi 21.
- **Ganti email lebih aman (D59).** Bila SMTP aktif, email yang diganti Pejuang sendiri belum langsung berlaku.
  - Tautan konfirmasi dikirim ke email baru (berlaku 24 jam, sekali pakai) dan dibuka di halaman web
    `/konfirmasi-email`. Email lama diberi tahu dengan alamat baru disamarkan.
  - Sampai dikonfirmasi, web (Pengaturan → Akun saya) dan aplikasi (Profil → Data pribadi) menampilkan
    "Menunggu konfirmasi" dengan tombol batal. Maksimal 3 permintaan per jam.
  - Setelah dikonfirmasi, email login berubah, semua sesi lama dikeluarkan, dan email lama diberi tahu.
  - Ganti email oleh SDM (form atau impor) tetap langsung berlaku, tetapi kini mengeluarkan semua sesi Pejuang itu
    dan memberi tahu email lama dan baru.
  - Endpoint `POST /v1/auth/email/confirm` dan `DELETE /v1/me/email-change`; `/v1/me` memuat `pending_email`;
    migrasi 20.
- **Lupa kata sandi lewat email (D30).** Di halaman masuk web dan aplikasi ada tautan **Lupa kata sandi?**. Pejuang
  menulis NIPY, email, atau nomor WA, lalu tautan dikirim ke email akunnya. Kata sandi baru dibuat di halaman web
  `/reset-kata-sandi`.
  - Jawaban selalu sama, baik akun ada atau tidak, agar tidak membocorkan akun yang terdaftar. Dibatasi 3 permintaan
    per jam per akun dan 10 per jam per alamat IP.
  - Tautan sekali pakai, berlaku 1 jam, dan tautan baru membatalkan yang lama. Setelah reset, kunci akun dibuka, semua
    sesi dikeluarkan, dan verifikasi dua langkah tetap diminta saat masuk.
  - Pemberitahuan keamanan dikirim ke email saat kata sandi diganti atau direset.
  - Endpoint `POST /v1/auth/password/forgot` dan `POST /v1/auth/password/reset`; migrasi 19.
- **Pengaturan email (SMTP) untuk super admin.** Kartu **Pengaturan → Email (SMTP)** mengatur host, port (465 atau 587,
  TLS wajib), akun, dan pengirim. Saat disimpan, email uji dikirim ke akun super admin dan pengaturan baru hanya
  tersimpan bila email uji terkirim. Kata sandi SMTP disimpan terenkripsi dan tidak pernah ditampilkan kembali. Endpoint
  `GET`/`PUT`/`DELETE /v1/admin/settings/smtp`.
- **Penyimpanan berkas di SeaweedFS (S3).** Selfie presensi, foto profil, dan lampiran pengumuman bisa disimpan di
  bucket privat S3 (SeaweedFS), tetap dienkripsi oleh API dan tanpa URL bucket untuk browser atau HP. Rekap XLSX dan
  template impor tidak ikut disimpan; keduanya langsung diunduh.
  - Super admin mengaktifkannya di **Pengaturan → Penyimpanan S3**; konfigurasi baru tersimpan setelah uji
    tulis/baca/hapus berhasil, dan secret key disimpan terenkripsi.
  - Berkas lama dipindah dari `SELFIE_DIR` dengan tombol sinkronisasi yang bisa dilanjutkan; salinan disk tidak dihapus
    otomatis. Endpoint dan bucket aktif dikunci.
  - Backup dan uji restore ikut menyalin isi bucket. Panduan: `backend/deploy/seaweedfs-storage.md`.
- **Impor Pejuang sekaligus menyiapkan struktur, shift, dan lokasi (D56).** Template impor kini punya kolom **Peran**
  (Kepala Subdiv, Kepala Divisi, Pejuang), **Shift**, dan **Lokasi 1–3**, serta sheet **Lokasi** dan **Shift**.
  - Cabang, divisi, dan subdiv yang belum ada cukup ditulis namanya, lalu dibuat otomatis dengan kode dari namanya.
  - Lokasi dan shift baru ditulis di baris kosong sheet-nya, lalu langsung bisa dipilih di sheet Pejuang.
  - Kolom Peran memasang kepala unit; role akun kepala baru dan kepala lama ikut menyesuaikan.
  - Semuanya diperiksa di pratinjau, yang juga menampilkan daftar data baru, dan disimpan dalam satu transaksi.
- **Lampiran pengumuman tampil langsung dan dipindai virus.** Gambar dan PDF lampiran langsung terlihat di pop-up dan
  detail pengumuman (aplikasi) serta di pratinjau dan form pengumuman (web), tanpa perlu mengetuk nama berkas; PDF bisa
  digeser per halaman dan ada tombol **Perbesar**. Setiap lampiran dipindai ClamAV sebelum disimpan: berkas berbahaya
  dijawab `FILE_UNSAFE` (422), dan bila pemindai tidak tersedia unggahan ditunda dengan `FILE_SCAN_UNAVAILABLE` (503).
- **Lampiran pengumuman (D54).** SDM bisa melampirkan sampai 5 berkas PDF, JPEG, atau PNG (maks 5 MB per berkas) pada
  pengumuman. Lampiran tampil di pop-up dan detail pengumuman di aplikasi, dan dibuka langsung di dalam aplikasi (PDF
  dirender per halaman) tanpa aplikasi lain. Berkas disimpan terenkripsi dan hanya bisa diunduh sasaran pengumuman yang
  sedang tayang. Endpoint `POST`/`DELETE /v1/admin/announcements/{id}/attachments`,
  `GET /v1/announcements/{id}/attachments/{attachment_id}`; migrasi 18.
- **Layar terang otomatis saat cahaya kurang (D55).** Kamera presensi dan pendaftaran wajah mengukur terang gambar;
  bila terlalu gelap, layar HP dibuat terang penuh dan sekitar bingkai wajah menjadi putih sebagai lampu wajah.
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
- **Kata sandi minimal 6 karakter (D70)**, sebelumnya 10. Kata sandi umum tetap ditolak.
- Peringatan kesiapan periode ubar kini menyebut presensi yang belum absen pulang akan ditutup otomatis paling lambat
  esok pagi; tautan Review Presensi hanya untuk presensi yang menunggu review.
- **Lama telat lebih mudah dibaca (D62).** Telat 60 menit atau lebih ditampilkan "1 jam 15 menit" di monitor
  presensi, Review Presensi, rekap (kolom **Lama telat**), serta Beranda, riwayat, dan hasil presensi di aplikasi.
  Hitungan potongan payroll tidak berubah.
- **Data anak hanya diubah SDM (D56).** Di aplikasi, Profil → Data anak kini hanya menampilkan data dan mengarahkan
  Pejuang ke SDM bila ada yang perlu diubah. Endpoint `PUT /v1/me/children` dihapus; SDM mengubah data anak lewat form
  Pejuang di web atau impor xlsx.
- **Lampiran pengumuman dikompres.** Foto diubah ukurannya (sisi terpanjang maks 2560 px, maks 24 MP), arah EXIF
  diterapkan, dan metadata (termasuk lokasi) dibuang; berkas lalu dikompres gzip sebelum dienkripsi bila hasilnya lebih
  kecil. Isi PDF tidak diubah.
- **Pop-up pengumuman tampil ulang harian (D54).** Setelah ditandai "Sudah baca", pop-up yang punya tanggal berakhir
  muncul lagi sekali sehari saat aplikasi dibuka, sampai berakhir, hanya dengan tombol **Tutup** (tanpa hitung mundur
  dan tanpa "Sudah baca" lagi). Pop-up tanpa tanggal berakhir tidak muncul lagi setelah dibaca.
- **Deteksi wajah lebih mudah untuk Pejuang lanjut usia (D55).** Bingkai wajah lebih toleran terhadap jarak, posisi,
  dan kemiringan kepala (sampai 20°); posisi cukup ditahan sebentar; mata yang cenderung sipit tetap terbaca terbuka;
  dan toleh kepala cukup 20°. Server ikut melonggarkan ukuran dan posisi wajah di foto, kemiripan antarselfie
  pendaftaran (0.45), dan batas toleh. Ambang pencocokan identitas presensi (0.50/0.46) tetap, agar presensi tidak bisa
  diwakilkan orang lain.
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
- **Kode versi di kartu Versi aplikasi Android mengikuti rumus rilis.** 0.14.0 kini dihitung 14000
  (major·1.000.000 + minor·1.000 + patch), sama dengan versionCode aplikasi. Sebelumnya dihitung 1400 sehingga
  pemberitahuan versi baru dan versi minimum tidak akan berfungsi. Server kini menolak kode yang tidak cocok dengan
  nama versi.
- **Aplikasi sebelum login tidak lagi ditolak `API_ACCESS_DENIED`.** Lupa kata sandi, footer layar masuk, dan cek
  versi dari aplikasi kini diteruskan karena aplikasi selalu mengirim `X-App-Version`.
- Fitur layar tegak ditandai opsional di manifest agar Play Console tidak membatasi tablet dan perangkat lain.
- **Pengumuman lebih andal muncul di aplikasi.** Pop-up yang ditunda dengan **Nanti saja** muncul lagi saat aplikasi
  dibuka kembali, tidak lagi tertahan sampai aplikasi ditutup penuh. Selama aplikasi terbuka, pengumuman dimuat ulang
  tiap 2 menit, dan bila gagal memuat (misalnya sinyal sesaat hilang) dicoba lagi setelah 15 detik.
- **Ikon mata di semua kolom kata sandi.** Web (masuk, ganti kata sandi, wajib ganti kata sandi, konfirmasi ganti email)
  dan aplikasi (masuk, ganti kata sandi, konfirmasi ganti email) kini punya ikon mata untuk melihat kata sandi yang
  diketik. Di web, isian kembali tersembunyi saat form dikirim.
- Pengumuman baru kini muncul di aplikasi setiap kali aplikasi dibuka kembali dari latar belakang. Sebelumnya hanya
  dimuat saat aplikasi pertama kali dibuka, sehingga pengumuman baru tidak muncul sampai aplikasi ditutup penuh.
- Batas 72 jam kata sandi awal kini benar-benar tersimpan untuk Pejuang baru yang ditambahkan lewat form. Sebelumnya
  hanya tersimpan saat reset kata sandi, sehingga kata sandi awal Pejuang baru tidak kedaluwarsa.

### Perhatian saat memperbarui
- Migrasi 000026 menambah kolom pulang otomatis di `attendances`. Jalankan `deploy.sh` seperti biasa; job
  `mark-absent` yang sudah terjadwal langsung ikut menutup presensi lupa absen pulang.
- Lupa kata sandi lewat email butuh `PUBLIC_WEB_URL` di `.env` server (alamat web admin, wajib `https`, contoh
  `https://hris.albahjah.or.id`) dan SMTP yang diatur super admin di panel. Tanpa keduanya, permintaan lupa kata sandi
  tetap dijawab tetapi tidak ada email yang dikirim.
- Sebelum mengaktifkan penyimpanan S3, isi `STORAGE_BACKUP_REMOTE` (remote rclone yang membaca bucket) di
  `/opt/hris/deploy.env`; tanpa itu backup gagal tertutup saat S3 aktif. Jangan hapus `SELFIE_DIR` dan jangan ganti
  `FIELD_ENCRYPTION_KEY_PATH` sampai migrasi selesai serta backup dan restore terverifikasi.
- Lampiran pengumuman wajib dipindai: pasang dan jalankan clamd di server sesuai `backend/deploy/clamav.md`, lalu isi
  `CLAMAV_ADDRESS` (socket Unix absolut atau `127.0.0.1:3310`). Tanpa itu, unggahan lampiran dijawab
  `FILE_SCAN_UNAVAILABLE` dan lampiran lama yang belum pernah dipindai tidak bisa dibuka.
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
