# CHECKLIST: HRIS Pejuang

Centang `[x]` saat selesai. Urutan mengikuti `ROADMAP.md`. Acuan tunggal dan status terbaru ada di `MASTER.md` §13.3.

## Fase 0: Demo Web Admin (Senin 2 Nov - Jumat 6 Nov 2026) — Selesai

### Senin: Fondasi
- [x] Repo Git + struktur folder (`backend/`, `web/`, `mobile/`)
- [x] Database PostgreSQL native (D12; tanpa Docker/PostGIS/pgvector sesuai D20, D22, D3)
- [x] Jalankan migrasi awal (`backend/migrations/000001_core.up.sql`)
- [x] Project Go: config, logger, koneksi DB, health check `/healthz` dan `/readyz`
- [x] Project Next.js + komponen Tailwind + TanStack Query (D14)
- [x] Layout web: sidebar, header, halaman menu
- [x] Login: `POST /v1/auth/web/login` + JWT + middleware role
- [x] Seed: akun SDM (`cmd/seed`)

### Selasa: Master Data
- [x] CRUD Divisi (API + UI)
- [x] CRUD Subdiv (API + UI, pilih divisi)
- [x] CRUD Pejuang (API + UI, pilih subdiv)
- [x] Cari, filter, paginasi di tabel
- [x] Validasi form + pesan error jelas
- [x] Data dummy realistis (`cmd/seed`)

### Rabu: Dashboard
- [x] Endpoint `GET /v1/admin/dashboard/org-chart` dan `summary`
- [x] Kartu ringkasan (total pejuang, divisi, subdiv, hadir hari ini)
- [x] Grafik Pejuang per subdiv
- [x] Grafik subdiv per divisi
- [x] Grafik langsung berubah setelah tambah/hapus Pejuang

### Kamis: Pengumuman & Laporan
- [x] CRUD pengumuman (judul, isi, tipe, jadwal, target divisi)
- [x] Pratinjau pop-up/banner di web
- [x] Data dummy presensi 1 bulan (`devsim`)
- [x] Halaman rekap presensi (filter bulan/divisi)
- [x] Halaman laporan upah sementara berdasarkan presensi (diganti aturan berversi di Fase 5, D29)
- [x] Ekspor CSV (aman dari formula injection)

### Jumat: Finalisasi (tanpa fitur baru)
- [x] Rapikan UI, ikon, warna, teks
- [x] Jalankan alur demo 2x tanpa error di server development

## Fase 1: Fondasi Produksi — Selesai
- [x] Refresh token + rotasi + deteksi reuse + logout
- [x] Rate limit login (in-memory `Limiter`, D16; Redis menyusul bila multi-instance) + lockout
- [x] Audit log append-only (`REVOKE UPDATE, DELETE` untuk `hris_app`)
- [x] OpenAPI docs + versi `/v1` + tes kontrak TS & Kotlin (`TestMobileDTOsMatchOpenAPI`)
- [x] CI: lint + test + gitleaks + govulncheck + trivy (`.github/workflows/ci.yml`)
- [x] Skrip pengalihan mode produksi HTTPS (`backend/deploy/setup-production.sh`) + CLI bootstrap akun SDM pertama (`cmd/bootstrap`)
- [x] Backup DB & kunci otomatis terenkripsi (`backend/deploy/backup.sh`) + uji restore mingguan otomatis (`backend/deploy/restore-verify.sh`)
- [x] Hak akses baris (Subdiv/Kadiv hanya bawahannya; scope 404)

## Fase 2: Presensi — Selesai
- [x] CRUD lokasi kantor (radius/poligon di Go, D22) + peta Leaflet di web (D28)
- [x] CRUD shift + toleransi telat + hari libur
- [x] Device binding (registrasi + approval SDM/superadmin, D25, D32, D34) + ikatan kunci instalasi ECDSA P-256 (`public_key` & `device_proof`)
- [x] Enroll wajah (selfie dihitung server dengan YuNet + SFace ONNX, terenkripsi AES-GCM, D43) + `consents` + izin daftar wajah jarak jauh 24 jam
- [x] `clock-in` / `clock-out` dengan validasi geofence + WFA/hybrid/jam fleksibel (D23, D26) + cutoff harian (D31) + job `mark_absent`
- [x] Pencocokan wajah 1:1 di Go (tanpa pgvector, D3) + tolak selfie replay + liveness 2 foto `selfie`+`gesture` (D51)
- [x] Verifikasi perangkat berjenjang di server, web admin (`/perangkat`), dan mobile: Play Integrity (jalur A), Android Key Attestation X.509 (jalur B), persetujuan manual berbatas 90 hari (jalur D, `integrity_manual` +30 → `review`), dan kiosk kantor dalam geofence (jalur E)
- [x] Skor risiko + status `review` + `attendance_attempts`
- [x] Halaman Review Absen (termasuk lihat selfie) + Monitor Absen untuk SDM
- [x] Rekap & laporan presensi rinci dari data nyata + retensi selfie 90 hari (`purge_selfies`)
- [x] Uji otomatis skenario serangan (`internal/attendance/attack_scenarios_test.go`): fake GPS, foto diam, emulator, root, replay selfie & challenge, kunci perangkat tidak cocok, Key Attestation, dan kiosk di luar kantor
- [ ] Kalibrasi lapangan di 3 HP Android 10 kelas bawah (dijalankan dengan perangkat fisik saat uji lapangan/pilot)

## Fase 3: Aplikasi Mobile
- [x] Setup project (`mobile/`) + navigasi bawah 5 tab (siap build `.aab` via Android Studio ke Play Store, D52)
- [x] Login (+MFA) + simpan refresh token aman di Android Keystore
- [x] Profile, Data Pribadi (`FLAG_SECURE`), Ganti Kata Sandi, Ukuran Huruf
- [x] Modul deteksi native: mock location, root, emulator, app fake GPS (`DeviceChecks.kt`, dikirim sebelum challenge)
- [x] Kamera + liveness (`FaceFit` oval + toleh kepala, kirim `selfie` & foto `gesture`, D51); embedding dihitung di server (D42)
- [x] Layar presensi + status lokasi/perangkat/wajah
- [x] Riwayat & rekap presensi harian/bulanan (rekap periode cutoff menunggu D31)
- [ ] Notifikasi FCM (pop-up & banner sudah diambil saat app dibuka; push FCM menunggu Firebase)
- [x] Pop-up & banner pengumuman + timer baca ditegakkan server (`min_read_seconds`, D27)
- [ ] Uji `minSdk 26`, Android 8-14 di 3 HP kelas bawah

## Fase 4: Pengajuan & Approval (Sedang Berjalan)
- [x] Mesin approval generik (`approval_flow_steps`, `approvals`, pengaturan alur oleh SDM, D46)
- [x] Cuti & izin (form, lampiran, saldo fleksibel `statutory`/`custom`/`none`, D47)
- [ ] Pinjaman/kasbon (Subdiv → Kadiv, D48) — *belum dikerjakan*
- [ ] Mutasi (Subdiv → Kadiv → SDM) + job tanggal efektif — *belum dikerjakan*
- [x] Antrian approval di web & mobile
- [x] Timeline status di aplikasi & web

## Fase 5: Payroll & SP (Belum Dimulai)
- [ ] Keputusan aturan upah dari SDM lewat panel berversi (D29); hapus rumus sementara di `internal/report`
- [ ] Cutoff periode (tanggal 25 & harian 04:00) + kunci setelah finalize + item penyesuaian (D31)
- [ ] Periode payroll + generate draft + komponen lembur, potongan telat/alpha, cicilan pinjaman
- [ ] Finalisasi maker-checker + mark-paid
- [ ] Slip PDF + unduh di aplikasi
- [ ] Surat Peringatan SP1-SP3 + acknowledge di akun Pejuang

## Fase 6: Rilis (Belum Dimulai)
- [ ] Email akun lewat SMTP yang diatur super admin, lupa sandi, ganti email (D30)
- [ ] Uji beban jam masuk (simulasi ribuan absen bersamaan)
- [ ] Audit keamanan (OWASP dasar, penetration test ringan)
- [ ] Kebijakan privasi + persetujuan data wajah & lokasi
- [ ] Pelatihan SDM + panduan pengguna
- [ ] Pilot 1 divisi (2 minggu)
- [ ] Rilis bertahap seluruh Pejuang lewat Google Play Store (D52)
- [ ] Monitoring & alert (uptime, error, latensi absen)

## Keputusan yang Harus Dikunci
- [x] Arti "ubar" = upah (D50)
- [x] Rumus potongan telat/alpha & lembur diatur SDM lewat panel berversi (D29)
- [x] Pinjaman/kasbon otomatis memotong ubar lewat cicilan (D48)
- [x] Alur approval cuti default Kadiv → SDM dan dapat diatur SDM (D46)
- [x] Kebijakan HP tanpa Google Play Services didukung lewat jalur berjenjang A–E (D24)
- [ ] Ambang skor risiko & face score (nilai awal ada, menunggu kalibrasi di HP nyata)
- [ ] Retensi resmi selfie & data lokasi (retensi teknis 90 hari sudah jalan, menunggu konfirmasi legal)
