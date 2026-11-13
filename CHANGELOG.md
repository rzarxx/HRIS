# Catatan Perubahan

Perubahan penting yang terlihat oleh Pejuang, SDM, atau pengelola server. Keputusan desain lengkap ada di
`MASTER.md` §14 (nomor D), kontrak API di `backend/api/openapi.yaml`. Format mengikuti
[Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/).

## Belum dirilis

### Ditambahkan
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
- Tes integrasi cuti tidak lagi gagal saat tanggal ujinya sama dengan hari ini.

### Perhatian saat memperbarui
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
