# CHECKLIST: HRIS Pejuang

Centang `[x]` saat selesai. Urutan mengikuti `ROADMAP.md`.

## Fase 0: Demo Web Admin (Senin 2 Nov - Jumat 6 Nov 2026)

### Senin: Fondasi
- [ ] Repo Git + struktur folder (`backend/`, `web/`, `docs/`)
- [ ] Docker Compose: PostgreSQL (+PostGIS, pgvector), Redis
- [ ] Jalankan migrasi dari `QUERY.sql` (tabel inti saja dulu)
- [ ] Project Go: config, logger, koneksi DB, health check `/healthz`
- [ ] Project Next.js + Refine/React-Admin + Tailwind
- [ ] Layout web: sidebar, header, halaman kosong per menu
- [ ] Login: `POST /v1/auth/login` + JWT + middleware role
- [ ] Seed: 1 akun SDM

### Selasa: Master Data
- [ ] CRUD Divisi (API + UI)
- [ ] CRUD Subdiv (API + UI, pilih divisi)
- [ ] CRUD Pejuang (API + UI, pilih subdiv)
- [ ] Cari, filter, paginasi di tabel
- [ ] Validasi form + pesan error jelas
- [ ] Data dummy realistis (3 divisi, 8+ subdiv, 40+ pejuang)

### Rabu: Dashboard
- [ ] Endpoint `GET /v1/admin/dashboard/org-chart`
- [ ] Kartu ringkasan (total pejuang, divisi, subdiv, hadir hari ini)
- [ ] Grafik Pejuang per subdiv
- [ ] Grafik subdiv per divisi
- [ ] Grafik langsung berubah setelah tambah/hapus Pejuang

### Kamis: Pengumuman & Laporan
- [ ] CRUD pengumuman (judul, isi, tipe, jadwal, target divisi)
- [ ] Pratinjau pop-up/banner di web
- [ ] Data dummy presensi 1 bulan
- [ ] Halaman rekap presensi (filter bulan/divisi)
- [ ] Halaman laporan upah berdasarkan presensi (rumus sederhana)
- [ ] Ekspor Excel/CSV (jika sempat)

### Jumat: Finalisasi (tanpa fitur baru)
- [ ] Rapikan UI, ikon, warna, teks
- [ ] Jalankan alur demo 2x tanpa error
- [ ] Rekaman layar cadangan
- [ ] Slide: masalah, solusi, arsitektur, demo, roadmap, kebutuhan
- [ ] Siapkan jawaban: jadwal app HP, keamanan absen, biaya server

## Fase 1: Fondasi Produksi
- [ ] Refresh token + logout
- [ ] Rate limit login
- [ ] Audit log dasar
- [ ] OpenAPI docs + versi `/v1`
- [ ] CI: lint + test + build
- [ ] Deploy staging (Docker + Nginx + HTTPS)
- [ ] Backup DB otomatis + uji restore
- [ ] Hak akses baris (Subdiv/Kadiv hanya bawahannya)

## Fase 2: Presensi
- [ ] CRUD lokasi kantor (radius/poligon) + peta di web
- [ ] CRUD shift + toleransi telat
- [ ] Device binding (registrasi + approval SDM)
- [ ] Enroll wajah (HP → embedding → server)
- [ ] `clock-in` / `clock-out` dengan validasi geofence
- [ ] Pencocokan wajah (pgvector)
- [ ] Verifikasi Play Integrity di server
- [ ] Skor risiko + status `review`
- [ ] Halaman Review Absen untuk SDM
- [ ] Rekap & laporan presensi dari data nyata
- [ ] Uji di 3 HP Android 10 kelas bawah
- [ ] Uji skenario serangan: fake GPS, foto, emulator, root

## Fase 3: Aplikasi Mobile
- [ ] Setup project + navigasi bawah
- [ ] Login + simpan token aman
- [ ] Profile
- [ ] Modul native: mock location, root, app fake GPS
- [ ] Kamera + liveness + embedding on-device
- [ ] Layar presensi + indikator geofence
- [ ] Riwayat & rekap presensi
- [ ] Notifikasi FCM
- [ ] Pop-up & banner pengumuman
- [ ] Uji `minSdk 26`, Android 8-14

## Fase 4: Pengajuan & Approval
- [ ] Mesin approval generik (tabel, service, notifikasi)
- [ ] Cuti & izin (form, lampiran, saldo cuti)
- [ ] Kasbon (Subdiv → Kadiv)
- [ ] Mutasi (Subdiv → Kadiv → SDM) + job tanggal efektif
- [ ] Antrian approval di web & mobile
- [ ] Timeline status di aplikasi

## Fase 5: Payroll & SP
- [ ] Keputusan aturan upah dari SDM (tertulis)
- [ ] Periode payroll + generate draft
- [ ] Komponen: lembur, potongan telat/alpha, potongan kasbon
- [ ] Finalisasi + kunci periode
- [ ] Slip PDF + unduh di aplikasi
- [ ] Surat Peringatan SP1-SP3 + tampil di akun Pejuang

## Fase 6: Rilis
- [ ] Uji beban jam masuk (simulasi ribuan absen bersamaan)
- [ ] Audit keamanan (OWASP dasar, penetration test ringan)
- [ ] Kebijakan privasi + persetujuan data wajah & lokasi
- [ ] Pelatihan SDM + panduan pengguna
- [ ] Pilot 1 divisi (2 minggu)
- [ ] Rilis bertahap seluruh Pejuang
- [ ] Monitoring & alert (uptime, error, latensi absen)

## Keputusan yang Harus Dikunci
- [ ] Arti "ubar" = upah?
- [ ] Rumus potongan telat/alpha & lembur
- [ ] Kasbon otomatis memotong upah, maks cicilan
- [ ] Alur approval cuti (sampai Kadiv atau tidak)
- [ ] Kebijakan HP tanpa Google Play Services
- [ ] Ambang skor risiko & face score
- [ ] Retensi selfie & data lokasi
