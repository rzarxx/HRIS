# Aplikasi Android HRIS Pejuang

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
  dan Profil. Pengajuan dan Slip upah masih menampilkan "Segera tersedia" sampai API Fase 4 dan 5 ada.
- Presensi: total jam kerja hari ini dan bulan berjalan, tombol absen masuk/pulang, dan riwayat 31 hari terakhir.
- Profil: kontak, data pribadi (hanya baca), status HP dan wajah, ganti kata sandi
  (`POST /v1/auth/password/change`), versi aplikasi, dan keluar. Layar Profil, Data pribadi, Ganti kata sandi,
  dan Slip upah memakai `FLAG_SECURE`.

Belum: Play Integrity (butuh akun Play Console), deteksi root/emulator/fake GPS app selain `isMock`,
notifikasi FCM, pengajuan, dan slip upah.

Bila aplikasi tertutup tiba-tiba, ringkasan galatnya (tanpa token atau data pribadi) muncul saat aplikasi dibuka
lagi, dengan tombol **Salin laporan** untuk dikirim ke tim.

## Mendapatkan APK

Setiap push menjalankan job CI `mobile` yang membangun APK debug, menjalankan tes unit, dan merender screenshot
tiap layar dengan Paparazzi (artefak `hris-pejuang-screenshots`, untuk meninjau tampilan tanpa HP). APK-nya ada di
halaman run GitHub Actions, bagian **Artifacts**, dengan nama `hris-pejuang-debug-apk` (disimpan 14 hari).
Pasang ke HP dengan mengizinkan instalasi dari sumber tidak dikenal, atau lewat `adb install`.

Build debug memakai `applicationId` `id.pejuang.hris.debug` dan menampilkan kolom **Alamat server** di layar
masuk, mis. `http://192.168.1.10:8080`. Build rilis memakai alamat dari `-Phris.apiBaseUrl=https://...` dan
tidak bisa diganti.

Build lokal (butuh JDK 17 dan Android SDK, lihat di bawah):

```bash
cd mobile
gradle assembleDebug recordPaparazziDebug
```

Repo belum memuat Gradle wrapper; buat sekali dengan `gradle wrapper --gradle-version 8.11.1` bila memakai
Android Studio.

## Struktur

| Folder | Isi |
|---|---|
| `data/` | DTO JSON, klien Retrofit, sesi dan token, penyimpanan Keystore, terjemahan error |
| `face/` | Pelacak liveness (Kotlin murni, ada tes unit), kamera CameraX + ML Kit, pengolah foto |
| `location/` | Lokasi GPS saat absen |
| `ui/` | Tema, komponen, dan layar Compose. Tiap layar punya versi `*Content` tanpa jaringan untuk screenshot |

Dependensi disusun manual di `HrisApp`, belum memakai Hilt (D44).

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
- Setiap challenge berisi `liveness_action` (`blink`, `turn_left`, `turn_right`, atau `smile`). Minta Pejuang
  melakukan gerakan itu, lalu kirim `liveness_passed` dan `liveness_action` yang sama.
- Daftar wajah (F7): bila `GET /v1/me` mengembalikan `face_enrolled: false`, buka layar daftar wajah. Minta
  challenge `face_enroll`, ambil 3 sampai 5 selfie berbeda, lalu kirim `multipart/form-data` ke
  `POST /v1/face/enroll`: bagian `payload` (JSON berisi `sample_sha256`, `liveness_*`, `consent_version`, dan
  lokasi) dan berkas `samples`. Pendaftaran di luar area kantor memerlukan izin satu kali dari SDM yang berlaku 24 jam.
- Absen masuk dan pulang juga `multipart/form-data`: bagian `payload` (isi absen ditambah `selfie_sha256`) dan
  berkas `selfie`.
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
| Haze | 1.2.2, blur bottom bar kaca. Blur aktif di Android 12+; Android 8–11 dan screenshot Paparazzi memakai lapisan putih pengganti |

## Perlu dicek di HP sungguhan

- Arah toleh kepala pada kamera depan. Saat ini `turn_left` dan `turn_right` sama-sama menerima toleh ke kiri
  atau kanan selama sudutnya lebih dari 25 derajat (`LivenessTracker`).
- Ambang mata tertutup, senyum, dan wajah lurus pada HP kelas bawah dan cahaya redup.
- Selfie tegak di berbagai merek HP setelah rotasi kamera diterapkan.

## Rilis dan pembaruan APK mandiri

Workflow `.github/workflows/release.yml` berjalan pada push tag `vMAJOR.MINOR.PATCH` dan menerbitkan
`hris-pejuang-release.apk` di GitHub Releases. Hanya rilis stabil yang dipakai; tag prarilis ditolak.
Versi `0.3.1`, misalnya, menghasilkan `versionCode` 3001 (`major * 1000000 + minor * 1000 + patch`;
minor dan patch 0–999, major maksimal 2100). Gunakan tag yang selalu lebih tinggi dari semua rilis sebelumnya.

Sebelum tag pertama, buat **kunci rilis yang tetap** di komputer yang aman (misalnya dengan `keytool`
atau Android Studio), simpan cadangannya di tempat aman, dan siapkan GitHub Actions secrets berikut:

| Secret | Isi |
|---|---|
| `ANDROID_KEYSTORE_BASE64` | Seluruh file keystore yang dienkode base64 tanpa perubahan byte |
| `ANDROID_KEYSTORE_PASSWORD` | Kata sandi keystore |
| `ANDROID_KEY_ALIAS` | Alias kunci rilis |
| `ANDROID_KEY_PASSWORD` | Kata sandi kunci |
| `HRIS_RELEASE_API_BASE_URL` | Alamat API tetap dengan HTTPS, misalnya `https://api.example.id/` |

Jangan masukkan kunci atau kata sandi ke repo. Semua rilis untuk instalasi yang sama wajib menggunakan
applicationId dan kunci yang sama. APK debug `id.pejuang.hris.debug` tidak dapat diperbarui menjadi APK
rilis `id.pejuang.hris`; pasang rilis sebagai aplikasi terpisah. Kehilangan atau mengganti kunci rilis
mencegah Android memasang pembaruan di atas aplikasi lama.

Setelah perubahan digabung dan CI hijau, buat tag pada commit yang sudah ditinjau dan push tag tersebut.
Workflow membangun APK, menjalankan tes unit, menyelaraskan dan menandatangani APK, memverifikasi tanda
tangan, lalu menerbitkan GitHub Release. Pembuatan tag memicu publikasi ke repo publik; jangan buat tag
percobaan di repo produksi. Jika secret belum dikonfigurasi, workflow gagal dan tidak menerbitkan APK.
Jangan ubah aset rilis yang sudah dibagikan; buat versi baru bila ada koreksi.

Aplikasi memeriksa `https://api.github.com/repos/rzarxx/HRIS/releases/latest` tanpa token. Repo harus
publik; token GitHub tidak boleh ditanam di APK. Aplikasi menerima hanya aset bernama tetap dengan digest
SHA-256 dari GitHub, mengunduhnya lewat DownloadManager, memeriksa ukuran, digest, nama paket, versi,
dan sertifikat, lalu meminta Android membuka installer. Pengguna harus mengizinkan sumber pemasangan
untuk aplikasi ini dan mengonfirmasi instalasi. Tidak ada pemasangan diam-diam. Unduhan dapat berlanjut
saat aplikasi ditutup; progres diperbarui ketika aplikasi aktif. Tidak diperlukan izin penyimpanan luas:
file unduhan sementara berada di folder khusus aplikasi, dan file terverifikasi di penyimpanan privat.

### Uji lokal dan di HP

1. Jalankan `gradle recordPaparazziDebug` dari `mobile/` untuk uji unit, perbandingan versi, validasi metadata, dan render layar.
   Debug tidak menjalankan updater karena nama paket/kunci debug berbeda dari rilis.
2. Untuk uji lengkap, gunakan **repo GitHub publik khusus pengujian**, bukan repo produksi; salin workflow
   dan atur lima secret di repo uji dengan keystore uji tersendiri. Workflow mengikat APK ke repo tempat
   workflow berjalan. Gunakan endpoint HTTPS pengujian yang sah untuk API aplikasi.
3. Terbitkan versi stabil pertama di repo uji, pasang APK rilisnya di HP/emulator Android 8+ lewat `adb install`.
   Terbitkan versi kedua dengan tag lebih tinggi dan **keystore uji yang sama**. Buka kembali aplikasi, pilih
   Unduh, periksa progres, izinkan pemasangan dari sumber ini bila diminta, lalu pilih Pasang dan setujui
   installer sistem. Tutup dan buka aplikasi saat unduhan berjalan untuk memeriksa pemulihan status.
4. Uji juga jaringan terputus, ruang habis, membatalkan unduhan, izin instalasi ditolak, checksum tidak
   cocok dan APK dengan kunci/nama paket berbeda. Pastikan tidak ada APK invalid yang ditawarkan untuk dipasang.
   Uji negatif terakhir dapat dilakukan dengan rilis uji terpisah; jangan gunakan kunci produksi.

Pengujian di atas membutuhkan Android SDK, dua APK bertanda tangan, dan perangkat atau emulator dengan
akses GitHub. CI debug tidak membuktikan bahwa rilis yang ditandatangani atau instalasi pada HP sudah berhasil.
Karena APK dipasang di luar Play Store, `PLAY_RECOGNIZED` bukan jaminan yang dapat diasumsikan: lihat D45
untuk ketergantungan verifikasi perangkat sebelum presensi produksi.

Contoh setelah perubahan yang disetujui masuk ke `main` dan kelima secret tersedia:

```bash
git checkout main
git pull --ff-only origin main
git tag -a v0.4.1 -m "Rilis Android 0.4.1"
git push origin v0.4.1
```

Periksa job **Android release** dan asset GitHub Release setelah push; perintah tag tidak membuktikan
build, tanda tangan, maupun pemasangan berhasil. Tag pertama yang berisi fitur updater harus sudah
dipasang sebagai APK rilis sebelum aplikasi tersebut dapat menawarkan pembaruan berikutnya.
