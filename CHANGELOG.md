# Catatan Perubahan

Perubahan penting yang terlihat oleh Pejuang, SDM, atau pengelola server. Keputusan desain lengkap ada di
`MASTER.md` §14 (nomor D), kontrak API di `backend/api/openapi.yaml`. Format mengikuti
[Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/).

## Belum dirilis

### Ditambahkan
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

### Diubah
- Challenge hanya meminta `turn_left`/`turn_right`; `blink` dan `smile` tidak diminta lagi karena tidak bisa
  dibuktikan server.
- Sampel daftar wajah harus menghadap lurus; sampel yang menoleh ditolak `FACE_NOT_DETECTED`.
- Serialisasi `request_hash` presensi dan pendaftaran wajah menambahkan `gesture_sha256` di akhir (contoh baru di
  `MASTER.md` §5.4).

### Diperbaiki
- Tes integrasi cuti tidak lagi gagal saat tanggal ujinya sama dengan hari ini.

### Perhatian saat memperbarui
- **Server dan aplikasi harus diperbarui bersamaan.** Server baru menolak aplikasi lama yang tidak mengirim foto
  gerakan (`VALIDATION_FAILED`, field `gesture`), dan server lama menolak payload aplikasi baru.
- Ambang toleh kepala (selfie lurus ≤ 0.20, foto gerakan ≥ 0.18, selisih ≥ 0.10, kemiripan ≥ 0.30) adalah nilai
  awal dari geometri wajah. Kalibrasi dengan foto dari HP Android 10 sungguhan sebelum pilot.
- Liveness ini menolak foto diam (cetak atau layar), tetapi video wajah yang diputar ulang masih bisa lolos.
  Verifikasi perangkat di server (jalur B, Key Attestation) masih dikerjakan terpisah.
