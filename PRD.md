# PRD: HRIS Pejuang

Versi 0.1 | Update: 2 Nov 2026 | Status: Draft

## 1. Ringkasan
Sistem HRIS dengan dua sisi:
- **Web Admin** (SDM, Subdiv, Kadiv): konfigurasi, approval, laporan.
- **Aplikasi Mobile** (Pejuang/karyawan): presensi, pengajuan, slip upah.

Keduanya memakai satu backend API.

## 2. Tujuan
1. Presensi akurat: geofence + verifikasi wajah + anti fake GPS.
2. Menghilangkan proses manual cuti, kasbon, mutasi, slip upah.
3. Memberi SDM data real-time (grafik struktur organisasi, laporan upah dari presensi).
4. Backend ringan, cepat saat jam masuk/pulang.

## 3. Istilah
| Istilah | Arti |
|---|---|
| Pejuang | Karyawan |
| SDM | Personalia (admin utama) |
| Subdiv | Sub divisi (atasan langsung) |
| Kadiv | Kepala divisi |
| Ubar | Diasumsikan upah/gaji (**konfirmasi**) |

## 4. Role & Hak Akses
| Role | Platform | Akses |
|---|---|---|
| Pejuang | Mobile | Profile, presensi, slip upah, cuti/izin, SP (lihat), pengumuman, ajukan mutasi & kasbon |
| Subdiv | Mobile + Web | Akses Pejuang + approve bawahan |
| Kadiv | Mobile + Web | Akses Pejuang + approve level divisi |
| SDM | Web (+ Mobile) | Semua akses Pejuang + kelola master data, pengumuman, SP, payroll, laporan |

## 5. Fitur

### 5.1 Dashboard Pejuang
1. **Profile**: data diri, jabatan, divisi/subdiv, foto.
2. **Presensi**: clock in/out (geofence + wajah), riwayat, laporan & rekap bulanan.
3. **Payroll**: daftar periode, detail komponen, unduh slip PDF.
4. **Cuti & Izin**: form, lampiran, status approval.
5. **Surat Peringatan**: lihat SP1-SP3 yang diterima.
6. **Pengumuman**: pop-up dan banner.
7. **Mutasi**: form, approval SDM + Subdiv + Kadiv.
8. **Kasbon**: form, approval Subdiv lalu Kadiv.

### 5.2 Dashboard SDM
1. Buat & kelola pengumuman (pop-up/banner, jadwal tayang, target).
2. Grafik jumlah Pejuang per subdiv dan jumlah subdiv per divisi.
3. Laporan upah berdasarkan presensi.
4. Master data: divisi, subdiv, pejuang, lokasi kantor (geofence), shift.
5. Kelola SP, persetujuan ganti perangkat, review presensi berisiko.

## 6. Aturan Approval
| Pengajuan | Alur |
|---|---|
| Cuti/Izin | Subdiv → SDM (Kadiv opsional, **konfirmasi**) |
| Kasbon | Subdiv → Kadiv (→ SDM untuk potongan upah, **konfirmasi**) |
| Mutasi | Subdiv asal → Kadiv → SDM (semua harus setuju) |

Mesin approval bersifat generik (tabel `approvals`), satu implementasi untuk semua jenis.

## 7. Persyaratan Presensi
- **Geofence**: titik/radius atau poligon per lokasi kantor, dicek di server (PostGIS).
- **Wajah**: embedding dibuat di HP, dicocokkan di server (cosine similarity). Liveness check di HP.
- **Anti fake GPS** (berlapis): mock flag, deteksi app fake GPS, Play Integrity (`MEETS_DEVICE_INTEGRITY`), deteksi root, validasi akurasi & lompatan lokasi, device binding, jam server.
- **Skor risiko** per absen; skor tinggi = status `review` untuk SDM, bukan langsung ditolak.
- Android minimum **8.0 (minSdk 26)**; target utama Android 10.

## 8. Non-Fungsional
- Respons absen < 1 detik (p95) pada beban jam sibuk.
- Backend Go, satu binary, RAM idle < 100 MB.
- HTTPS wajib, password bcrypt/argon2, JWT access pendek + refresh token.
- Data wajah & lokasi = data pribadi: enkripsi, retensi terbatas, persetujuan pengguna (UU PDP).
- Audit log untuk aksi SDM dan perubahan data sensitif.
- API versi `/v1`, dokumentasi OpenAPI.

## 9. Di Luar Lingkup (Fase Awal)
iOS, integrasi mesin fingerprint, integrasi bank/transfer gaji otomatis, BPJS/PPh21 otomatis (fase lanjut).

## 10. Metrik Sukses
- 95% absen selesai < 10 detik dari buka aplikasi.
- Absen palsu terdeteksi/ditandai > 95% pada uji internal.
- Waktu penyusunan rekap upah bulanan turun > 70%.

## 11. Risiko
| Risiko | Mitigasi |
|---|---|
| HP lama lambat saat liveness | Fallback ulang / status review |
| Fake GPS canggih | Berlapis + validasi server + review SDM |
| HP tanpa Google Play Services | Kebijakan: tidak didukung atau verifikasi manual |
| Aturan upah belum jelas | Putuskan sebelum modul payroll |

## 12. Pertanyaan Terbuka
1. Rumus potongan telat/alpha dan lembur?
2. Kasbon otomatis memotong upah? Berapa maks cicilan?
3. Siapa approver cuti: Subdiv saja atau sampai Kadiv?
4. Satu Pejuang boleh banyak lokasi kantor?
5. Apakah "ubar" = upah?
