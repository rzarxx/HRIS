# Aplikasi Android HRIS Yayasan Al Bahjah

Aplikasi untuk Pejuang: login, absen dengan lokasi dan wajah, riwayat, pengajuan, dan slip upah. Acuannya tetap
`MASTER.md` (§10.2 Mobile, §5.9 Kontrol Mobile, F5 Login Mobile). Stack: Kotlin, Jetpack Compose, Retrofit/OkHttp,
kotlinx.serialization, CameraX, ML Kit, FusedLocationProvider. `minSdk 26`. Pencocokan wajah dihitung server dari
selfie (D42), jadi aplikasi tidak memuat model pengenalan wajah. Play Integrity dan FCM menyusul.

## Yang sudah ada (D44)

- Masuk dengan email, kata sandi, dan kode MFA bila diminta; refresh token terenkripsi dengan kunci Android
  Keystore; sesi diperpanjang otomatis saat access token habis.
- Beranda: sapaan, presensi hari ini (jam masuk/pulang), tombol absen besar, status HP dan wajah.
- Persetujuan biometrik dan lokasi, lalu daftar wajah: challenge `face_enroll`, gerakan liveness, 3 selfie.
- Absen masuk dan pulang: challenge, gerakan liveness, selfie, lokasi GPS, kirim multipart.
- Navigasi bawah lima menu sesuai MASTER.md §10.2 dan UI kit Figma: Beranda, Presensi, Pengajuan, Slip upah,
  dan Profil. Slip upah masih menampilkan "Segera tersedia" sampai API Fase 5 ada.
- Pengajuan cuti/izin: hak cuti per jenis (UU, diatur SDM, atau tidak mengurangi hak cuti), riwayat dengan status, detail dengan
  timeline persetujuan, batalkan selama menunggu, dan form di lembar bawah dengan pemilih rentang tanggal.
  Galat dari server tampil di bawah kolomnya.
- Persetujuan untuk akun dengan izin `approval:act` (kepala subdiv, kepala divisi, SDM): antrean di tab
  Pengajuan, setujui dengan catatan atau tolak dengan alasan wajib. Jumlah yang menunggu tampil sebagai lencana
  di navigasi bawah dan di Beranda, dimuat ulang tiap tab Pengajuan dibuka (belum ada notifikasi push).
- Pengumuman dari SDM, diambil saat aplikasi dibuka: pop-up satu per satu (bisa ditunda dengan "Nanti saja"),
  banner di Beranda, dan daftar Pengumuman. Tombol "Sudah baca" menghitung mundur sesuai jeda yang diatur SDM;
  server mencatat waktu buka sekali dan menolak "Sudah baca" yang terlalu cepat (D27).
- Tarik ke bawah untuk memuat ulang di Beranda, Presensi, dan Pengajuan; kerangka abu-abu saat memuat; getar
  singkat saat presensi, pengajuan, atau keputusan berhasil; pesan singkat (snackbar) hasil tindakan.
- Tampilan Clean Minimalist / Soft UI (DESIGN.md §8): latar terang, kartu putih dengan bayangan lembut, hijau terang
  dengan teks gelap, huruf Plus Jakarta Sans (dibundel, lisensi OFL di `OFL-PlusJakartaSans.txt`). Beranda berisi
  satu kartu presensi hari ini dengan satu tombol besar dan akses cepat ke Riwayat, Cuti dan izin, Pengumuman, dan
  Slip ubar.
- Nama aplikasi **HRIS Yayasan Al Bahjah** dengan ikon logo PUTI Al Bahjah (dari `logoputi.png`, ikon adaptif berlatar
  putih). Footer "© {tahun} PUTI (Pusat Teknologi Informasi) Al Bahjah" di Beranda, Profil, dan layar masuk; teksnya
  diambil dari `/v1/branding` (diatur super admin di web) dan disimpan untuk tampilan luring.
- Ukuran huruf bisa dipilih di Profil → Ukuran huruf (Kecil, Normal, Besar, Sangat besar), tersimpan di HP dan
  dikalikan dengan ukuran huruf sistem.
- Presensi: total jam kerja hari ini dan bulan berjalan, tombol absen masuk/pulang, dan riwayat 31 hari terakhir.
- Profil: kontak, data pribadi (hanya baca), status HP dan wajah, ganti kata sandi
  (`POST /v1/auth/password/change`), versi aplikasi, dan keluar. Layar Profil, Data pribadi, Ganti kata sandi,
  dan Slip upah memakai `FLAG_SECURE`.

Belum: Play Integrity (butuh akun Play Console), deteksi root/emulator/fake GPS app selain `isMock`,
notifikasi FCM, pinjaman, mutasi, dan slip upah.

Bila aplikasi tertutup tiba-tiba, ringkasan galatnya (tanpa token atau data pribadi) muncul saat aplikasi dibuka
lagi, dengan tombol **Salin laporan** untuk dikirim ke tim.

## Build di Android Studio & Rilis ke Google Play Store

Aplikasi didistribusikan dan diperbarui langsung melalui **Google Play Store** (D52), sehingga izin
`REQUEST_INSTALL_PACKAGES` dan pengecekan rilis GitHub tidak lagi digunakan.

- Build debug memakai `applicationId` `id.pejuang.hris.debug` dan menampilkan kolom **Alamat server** di layar
  masuk, mis. `http://192.168.1.10:8080`.
- Build rilis memakai `applicationId` `id.pejuang.hris`, secara bawaan mengarah ke `https://api.albahjah.or.id`
  (bisa ditimpa dengan `-Phris.apiBaseUrl=https://...`), dan alamat server tidak bisa diganti di layar masuk.

### Langkah Build di Android Studio

1. Buka folder `mobile/` di Android Studio dan tunggu proses *Gradle Sync* selesai (atau buat wrapper sekali
   dengan `gradle wrapper --gradle-version 8.11.1`).
2. Sebelum rilis baru, naikkan `releaseVersionName` (format `x.y.z`) dan `releaseVersionCode` di
   `mobile/app/build.gradle.kts` (bawaan `1.0.0` / `1000000`).
3. Pilih menu **Build → Generate Signed App Bundle / APK...** → pilih **Android App Bundle**.
4. Gunakan keystore rilis yang tetap (jangan sampai hilang/berganti antarversi), pilih variant **`release`**,
   lalu unggah berkas `mobile/app/release/app-release.aab` ke **Google Play Console**.

Build dan tes lokal dari terminal (butuh JDK 17 dan Android SDK):

```bash
cd mobile
gradle assembleDebug recordPaparazziDebug bundleRelease
```

## Struktur

| Folder | Isi |
|---|---|
| `data/` | DTO JSON, klien Retrofit, sesi dan token, penyimpanan Keystore, terjemahan error |
| `face/` | Pelacak liveness dan pemeriksaan posisi wajah `FaceFit` (Kotlin murni, ada tes unit), kamera CameraX + ML Kit, pengolah foto |
| `location/` | Lokasi GPS saat absen |
| `ui/` | Tema, komponen, dan layar Compose. Tiap layar punya versi `*Content` tanpa jaringan untuk screenshot |

Dependensi disusun manual di `HrisApp`, belum memakai Hilt (D44).

## Kamera presensi

Wajah harus benar-benar pas di oval sebelum liveness dimulai dan sebelum foto diambil. Wajah yang baru
terlihat sebagian di tepi bingkai tidak lolos.

- Oval (`faceOval()` di `face/FaceFit.kt`): 70% lebar layar, tinggi 1,32 kali lebarnya, pusat di 40% tinggi
  layar. Fungsi yang sama dipakai untuk menggambar (`CameraOverlay`) dan memeriksa, jadi yang dilihat
  Pejuang sama dengan yang dinilai.
- Kotak wajah ML Kit ada di koordinat frame analisis yang sudah ditegakkan (`rotationDegrees` 90/270 menukar
  lebar dan tinggi). `mapToView()` memetakannya ke layar seperti `PreviewView.ScaleType.FILL_CENTER` dan
  mencerminkannya karena pratinjau kamera depan dicerminkan. Pratinjau, analisis, dan foto sama-sama 4:3.
- Syarat pas (`checkFit()`): pusat wajah di dalam oval; lebar wajah 55–85% lebar oval; pusat wajah paling
  jauh 12% lebar oval (mendatar) dan 10% tinggi oval (tegak) dari pusat oval; elips wajah di dalam oval
  dengan toleransi 5%; kepala tidak miring (`headEulerAngleZ`) atau menunduk/mendongak (`headEulerAngleX`)
  12 derajat atau lebih.
- Pesan arahan: "Masukkan wajah ke dalam bingkai", "Dekatkan wajah", "Jauhkan sedikit", "Geser ke tengah",
  "Tegakkan kepala", lalu "Tahan posisi". Saat pas oval berubah hijau dan cincin kemajuan terisi. Tidak ada
  animasi lain agar ringan di HP kelas bawah.
- `LivenessTracker`: posisi lurus awal dan posisi akhir sebelum foto harus pas 8 frame berturut-turut.
  Selama gerakan (menoleh, berkedip, tersenyum) cukup pusat wajah tetap di oval. Wajah yang keluar oval,
  hilang, atau bertambah membuat liveness diulang dari awal.
- Setelah `takePicture`, ML Kit memeriksa foto hasil jepretan (`photoFits()`): tepat satu wajah (wajah lain
  di bawah 25% luas wajah utama diabaikan, seperti server), lebar wajah minimal 22% lebar foto dan 45% lebar
  oval, pusatnya di 56% bagian tengah foto dan di dalam oval. Bila tidak, foto diambil ulang otomatis paling
  banyak 2 kali, lalu Pejuang diminta mengulang.
- Batas lebar foto memakai oval, bukan 30% lebar foto: di layar tinggi pratinjau `FILL_CENTER` memotong sisi
  kiri-kanan foto 4:3, sehingga wajah yang pas di oval hanya sekitar 24–38% lebar foto. Karena itu pula
  `setMinFaceSize` detektor analisis 0,2, di bawah wajah pas yang terkecil.
- Server menolak wajah utama yang lebarnya kurang dari 20% foto atau pusatnya di luar 60% bagian tengah foto
  (`FACE_NOT_DETECTED`, "Wajah terlalu kecil atau tidak di tengah bingkai").

### Liveness yang dibuktikan server (D51)

- Challenge hanya meminta toleh kepala (`turn_left`/`turn_right`). Saat `LivenessTracker` pertama kali melihat
  kepala menoleh (`gestureSeen`), frame analisis itu ditegakkan dan dikompresi menjadi **foto gerakan**. Bila
  wajah keluar oval, hilang, atau bertambah, foto itu dibuang dan gerakan diulang.
- Selfie lurus dan foto gerakan dikirim bersama (berkas `selfie`/`samples` dan `gesture`, hash di
  `gesture_sha256`). Server mengukur rasio toleh dari lima titik wajah YuNet: selfie harus lurus, foto gerakan
  harus menoleh jelas, dan keduanya orang yang sama. Gagal berarti `LIVENESS_FAILED`.
- Dua upaya wajah yang ditolak dalam 30 menit menyalakan `liveness_retry` (+25) di server.

### Pemeriksaan HP sebelum presensi

`security/DeviceRules.kt` (Kotlin murni, ada tes) dan `security/DeviceChecks.kt` memeriksa, sebelum challenge
diminta:

- root: berkas `su` di lokasi umum atau aplikasi root (Magisk, SuperSU, KernelSU, APatch);
- emulator: sidik build emulator (`generic`, `ranchu`, `goldfish`, `sdk_gphone`, Genymotion);
- aplikasi pemalsu lokasi yang umum. Daftar paketnya juga ada di `<queries>` `AndroidManifest.xml` agar
  terlihat di Android 11+; tes `manifestQueriesMatchRules` menjaga keduanya tetap sama.

Bila salah satunya terdeteksi, layar menampilkan pesan yang jelas dan challenge tidak diminta. Hasilnya juga
dikirim sebagai `signals` sehingga server ikut menolak. Aturannya sengaja konservatif: `Build.TAGS` berisi
`test-keys` tidak dipakai karena banyak HP murah yang sah dibangun begitu. Deteksi ini hanya saran; HP yang
dimodifikasi dengan rapi bisa menyembunyikannya sampai verifikasi perangkat di server selesai.

## Yang dipasang di PC pengembang

Pasang versi stabil terbaru saat memulai, lalu catat versinya di bagian akhir README ini.

| Perangkat lunak | Catatan |
|---|---|
| Android Studio (stabil) | Sudah termasuk JDK untuk Gradle. Tidak perlu memasang JDK terpisah. |
| Git | Untuk mengambil repo dan membuat branch `feat/...` seperti biasa. |
| Driver USB HP | Windows: *Google USB Driver* dari SDK Manager, atau driver dari pabrikan HP. |

Lewat **SDK Manager** di Android Studio:

| Komponen | Untuk apa |
|---|---|
| Android SDK Platform | Satu versi untuk `compileSdk` dan `targetSdk`, mengikuti batas *target API level* Google Play saat rilis |
| Android SDK Build-Tools | Versi terbaru |
| Android SDK Platform-Tools | `adb`, untuk memasang aplikasi ke HP |
| Android SDK Command-line Tools | Build dari terminal dan CI |
| Android Emulator + system image | Opsional. Kamera, GPS, dan Play Integrity tetap harus diuji di HP sungguhan. |
| NDK dan CMake | Nanti, saat modul deteksi native dibuat (§5.9) |

Pustaka (Compose, Retrofit, CameraX, ML Kit, lokasi) diambil otomatis oleh Gradle, tidak dipasang manual.

## Akun dan layanan

| Kebutuhan | Kapan |
|---|---|
| Akun Google Play Console | Sebelum Play Integrity. Ada biaya pendaftaran sekali dan verifikasi identitas yang bisa makan beberapa hari. |
| Proyek Firebase | Untuk notifikasi (FCM). File `google-services.json` jangan di-commit. |
| Play Integrity API | Diaktifkan di proyek Google Cloud yang terhubung ke Play Console |
| 3 HP Android kelas bawah | Untuk uji. `minSdk 26` berarti Android 8 ke atas. |

## Menghubungkan ke API

- Kontrak ada di `backend/api/openapi.yaml`. Model JSON ditulis di `data/Dto.kt` dengan penanda
  `// schema: Nama` di atas tiap kelas; tes backend `TestMobileDTOsMatchOpenAPI` gagal bila ada `@SerialName`
  yang tidak ada di skema itu (D44). Tambahkan penanda yang sama untuk kelas baru.
- Login: `POST /v1/auth/mobile/login` dengan `email`, `password`, `mfa_code` (bila akun memakai MFA), dan
  `device.device_hash` (SHA-256 heksadesimal, 64 karakter, stabil per HP). Responsnya `access_token`,
  `refresh_token`, `device_id`, dan `device_status`.
- Simpan `refresh_token` di Android Keystore, `access_token` cukup di memori. Perpanjang lewat
  `POST /v1/auth/mobile/refresh` sebelum access token habis (15 menit). Refresh token lama tidak boleh dipakai
  ulang, karena sesi langsung dicabut. Sesi berakhir 30 hari setelah login (D40).
- Kirim `Authorization: Bearer <access_token>` di semua request. Route presensi juga butuh `X-Device-Id`
  berisi `device_id` dari login.
- `device_status: pending` berarti HP menunggu persetujuan SDM. Login tetap boleh, tetapi absen ditolak
  `DEVICE_PENDING_APPROVAL`.
- Tampilkan pesan dari kode error di `MASTER.md` §7.4, misalnya `DEVICE_REVOKED` dan `ATTENDANCE_BLOCKED`.
- Wajah dihitung server dari selfie (D42). HP tidak menghitung atau mengirim embedding; ML Kit dipakai untuk
  mendeteksi wajah, memotong foto, dan liveness.
- Setiap challenge berisi `liveness_action` (`turn_left` atau `turn_right`, D51). Minta Pejuang menoleh, lalu
  kirim `liveness_passed`, `liveness_action` yang sama, dan foto saat menoleh (berkas `gesture` dan
  `gesture_sha256`).
- Daftar wajah (F7): bila `GET /v1/me` mengembalikan `face_enrolled: false`, buka layar daftar wajah. Minta
  challenge `face_enroll`, ambil 3 sampai 5 selfie lurus yang berbeda, lalu kirim `multipart/form-data` ke
  `POST /v1/face/enroll`: bagian `payload` (JSON berisi `sample_sha256`, `gesture_sha256`, `liveness_*`,
  `consent_version`, dan lokasi), berkas `samples`, dan berkas `gesture`. Pendaftaran di luar area kantor
  memerlukan izin satu kali dari SDM yang berlaku 24 jam.
- Presensi masuk dan pulang juga `multipart/form-data`: bagian `payload` (isi presensi ditambah `selfie_sha256`
  dan `gesture_sha256`), berkas `selfie`, dan berkas `gesture`. Server D51 menolak aplikasi lama yang tidak
  mengirim `gesture` (`VALIDATION_FAILED`), jadi server dan aplikasi diperbarui bersamaan.
- Selfie: JPEG yang sudah tegak (server tidak membaca orientasi EXIF), satu wajah, sisi terpanjang sekitar 640 px, maksimal 1 MB. Selalu ambil foto baru untuk setiap upaya, karena
  foto yang pernah dikirim ditolak `ATTENDANCE_BLOCKED`. Hapus foto dari HP setelah terkirim.
- `FACE_NOT_DETECTED` berarti foto perlu diambil ulang (cahaya, wajah terhalang, atau lebih dari satu orang).
  `FACE_SERVICE_UNAVAILABLE` (503) berarti mesin wajah server belum dipasang.

## Server development

- Server development masih HTTP. Build debug mengizinkan cleartext ke semua alamat karena alamat server diisi
  saat masuk (`src/debug/res/xml/network_security_config.xml`). Build rilis wajib HTTPS
  (`cleartextTrafficPermitted=false`).
- HP biasanya tidak bisa memakai baris `hosts` seperti laptop, jadi `api.hris.local` tidak akan ditemukan.
  Pakai alamat IP server, atau minta pengelola jaringan menambahkan nama itu di DNS lokal.
- Di server development, absen dan daftar wajah dapat diuji dengan `DEV_ALLOW_UNVERIFIED_ATTENDANCE=true`
  atau dengan konfigurasi Play Integrity eksperimental. Keduanya tidak boleh diaktifkan bersamaan.
  Produksi tetap menolak sampai pengikatan perangkat selesai (D36).

## Versi yang dipakai

| | Versi |
|---|---|
| Gradle | 8.11.1 |
| Android Gradle Plugin | 8.7.3 |
| Kotlin | 2.1.0 |
| JDK | 17 |
| `compileSdk` / `targetSdk` | 35 / 35. Naikkan mengikuti batas Google Play sebelum rilis |
| Compose BOM | 2024.12.01 |
| CameraX | 1.4.1 |
| ML Kit face detection | 16.1.7 (model dibundel) |
| ABI | Hanya `arm64-v8a` dan `armeabi-v7a`. Pustaka native dikompresi (`useLegacyPackaging`) agar diterima pemasang bawaan OEM; skrip tanda tangan rilis menolak APK yang menyimpang. Akibatnya APK tidak bisa dipasang di emulator x86 |
| Haze | 1.2.2, blur bottom bar kaca. Blur aktif di Android 12+; Android 8–11 dan screenshot Paparazzi memakai lapisan putih pengganti |

## Perlu dicek di HP sungguhan

- Arah toleh kepala pada kamera depan. Saat ini `turn_left` dan `turn_right` sama-sama menerima toleh ke kiri
  atau kanan selama sudutnya lebih dari 25 derajat (`LivenessTracker`).
- Ambang mata tertutup, senyum, dan wajah lurus pada HP kelas bawah dan cahaya redup.
- Selfie tegak di berbagai merek HP setelah rotasi kamera diterapkan.
- Oval hijau benar-benar sesuai posisi wajah di layar (pemetaan `FILL_CENTER` dan cermin kamera depan), dan
  ambang pas di oval tidak terlalu ketat di HP Android 10 kelas bawah.

## Catatan Rilis Google Play Store

- Pastikan **kunci penandatanganan rilis (keystore `.jks`)** disimpan dengan aman dan dicadangkan. Mengganti atau
  kehilangan kunci akan mencegah pembaruan aplikasi di Play Console (kecuali memakai *Play App Signing* dengan
  upload key reset).
- Build debug (`id.pejuang.hris.debug`) dapat dipasang berdampingan dengan build Play Store (`id.pejuang.hris`)
  untuk pengujian lokal tanpa menimpa versi produksi.
- Untuk menguji fitur presensi dan liveness di HP sungguhan sebelum rilis produksi, pastikan seluruh skenario
  `Perlu dicek di HP sungguhan` di atas sudah diverifikasi pada minimal 3 HP Android kelas bawah (`minSdk 26`).
