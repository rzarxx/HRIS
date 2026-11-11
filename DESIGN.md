# DESIGN: HRIS Pejuang

## 1. Arsitektur

```
Aplikasi HP (Kotlin / Flutter + modul native) ─┐
                                               ├─► API Go (REST/JSON, /v1) ─► PostgreSQL (PostGIS, pgvector)
Web Admin (Next.js + Refine/React-Admin)     ───┘                         └─► Redis (cache, rate limit)
                                                    │
                                                    ├─► FCM (push notifikasi)
                                                    ├─► Play Integrity (verifikasi di server)
                                                    └─► Object storage (selfie, slip PDF, lampiran)
```

Prinsip: semua aturan bisnis ada di API Go. Klien hanya menampilkan dan mengirim data.

## 2. Tech Stack
| Lapisan | Pilihan |
|---|---|
| Backend | Go, `chi` (router), `pgx` (driver), `sqlc` (query type-safe), `golang-migrate` |
| Database | PostgreSQL 16 + PostGIS + pgvector |
| Cache | Redis |
| Web admin | Next.js + Refine (atau React-Admin), Tailwind, Recharts |
| Mobile | Kotlin native atau Flutter + platform channel native untuk deteksi |
| Wajah | ML Kit (deteksi) + TFLite MobileFaceNet (embedding), liveness di HP |
| Notifikasi | FCM |
| Deploy | Docker Compose (api, postgres, redis, nginx) |
| Docs API | OpenAPI 3 (swag atau oapi-codegen) |

## 3. Struktur Folder

```
hris/
├─ backend/
│  ├─ cmd/api/main.go
│  ├─ internal/
│  │  ├─ auth/          # login, JWT, refresh, middleware role
│  │  ├─ org/           # divisi, subdiv
│  │  ├─ employee/
│  │  ├─ attendance/    # geofence, face, risk scoring
│  │  ├─ approval/      # mesin approval generik
│  │  ├─ leave/ cashadvance/ transfer/
│  │  ├─ payroll/
│  │  ├─ announcement/
│  │  ├─ warning/
│  │  ├─ report/
│  │  ├─ notify/        # FCM
│  │  └─ platform/      # db, redis, config, logger
│  ├─ migrations/
│  ├─ queries/          # file sqlc
│  └─ api/openapi.yaml
├─ web/                 # Next.js admin
├─ mobile/
├─ docs/                # dokumen ini
└─ docker-compose.yml
```

## 4. Autentikasi & Otorisasi
- Login email + password → `access_token` (JWT, 15 menit) + `refresh_token` (7-30 hari, disimpan hash di DB/Redis).
- Klaim JWT: `sub`, `employee_id`, `role`, `exp`.
- Middleware `RequireRole(...)` per grup route.
- Row-level rule di service: Subdiv hanya melihat bawahan subdiv-nya, Kadiv hanya divisinya, SDM semua.
- Mobile: endpoint presensi juga wajib `X-Device-Id` terdaftar & Play Integrity token.

## 5. Endpoint API (v1)

### Auth
`POST /v1/auth/login` · `POST /v1/auth/refresh` · `POST /v1/auth/logout`

### Pejuang (mobile)
| Method | Path | Fungsi |
|---|---|---|
| GET | /v1/me | Profile |
| PUT | /v1/me | Update data terbatas |
| POST | /v1/devices/register | Daftar perangkat |
| POST | /v1/face/enroll | Daftar wajah |
| POST | /v1/attendance/clock-in | Absen masuk |
| POST | /v1/attendance/clock-out | Absen pulang |
| GET | /v1/attendance | Riwayat |
| GET | /v1/attendance/summary?month= | Rekap |
| GET | /v1/payrolls · /v1/payrolls/{id}/slip | Daftar & PDF slip |
| POST/GET | /v1/leaves | Ajukan / daftar cuti |
| POST/GET | /v1/cash-advances | Kasbon |
| POST/GET | /v1/transfers | Mutasi |
| GET | /v1/warnings | SP saya |
| GET | /v1/announcements/active | Pop-up & banner |
| POST | /v1/announcements/{id}/read | Tandai dibaca |
| GET | /v1/notifications | Notifikasi |

### Approver (Subdiv, Kadiv, SDM)
`GET /v1/approvals/pending` · `POST /v1/approvals/{id}/approve` · `POST /v1/approvals/{id}/reject`

### Admin (SDM)
| Path | Fungsi |
|---|---|
| /v1/admin/divisions, /subdivisions, /employees | CRUD master data |
| /v1/admin/office-locations | CRUD geofence |
| /v1/admin/shifts | CRUD shift |
| /v1/admin/announcements | CRUD pengumuman |
| /v1/admin/warnings | Terbitkan SP |
| /v1/admin/devices/{id}/approve | Setujui ganti perangkat |
| /v1/admin/attendance/review | Antrian absen berisiko |
| /v1/admin/payroll/periods, /generate, /finalize | Payroll |
| /v1/admin/dashboard/org-chart | Data grafik |
| /v1/admin/reports/attendance, /reports/payroll | Laporan |

Format error seragam: `{ "error": { "code": "GEOFENCE_OUT", "message": "..." } }`.

## 6. Alur Validasi Absen (server)
1. Verifikasi JWT, device terdaftar & `approved`.
2. Verifikasi token Play Integrity (`MEETS_DEVICE_INTEGRITY`).
3. Cek geofence (PostGIS).
4. Cocokkan embedding wajah + cek flag liveness dari HP.
5. Hitung skor risiko.
6. Simpan; status `present`/`late`, atau `review` bila skor tinggi.
7. Push notifikasi hasil.

### Skor Risiko (awal, bisa disetel)
| Sinyal | Poin |
|---|---|
| Integrity hanya basic / lemah | +30 |
| Akurasi GPS > 50 m | +15 |
| Lompatan lokasi tidak masuk akal | +40 |
| Face score di bawah ambang atas tapi lolos | +20 |
| Liveness gagal berulang | +25 |
| Jam HP beda jauh dari jam server | +20 |
| Mock location / root / app fake GPS | **tolak langsung** |

Ambang: 0-29 lolos, 30-59 `review`, ≥ 60 ditolak. Angka ini titik awal; setel setelah uji lapangan.

## 7. Desain UI Web Admin

**Gaya:** bersih, padat informasi, dominan putih/abu, aksen hijau `#15803D` dipadu kuning `#FACC15`. Kuning hanya untuk unsur merek (logo, penanda menu aktif, avatar, garis kartu masuk): jangan dipakai untuk teks di atas putih atau untuk status, karena peringatan memakai amber. Teks di atas kuning memakai `#14532D`. Font Inter. Mode terang dulu.

**Layout:** sidebar kiri (ikon + label), header atas (cari, notifikasi, profil), konten kartu dengan sudut 8px.

**Menu sidebar**
1. Dashboard
2. Master Data (Divisi, Subdiv, Pejuang, Lokasi Kantor, Shift)
3. Presensi (Rekap, Review Absen)
4. Payroll
5. Pengajuan (Cuti, Kasbon, Mutasi)
6. Surat Peringatan
7. Pengumuman
8. Pengaturan

**Halaman prioritas demo**
| Halaman | Isi |
|---|---|
| Login | Form sederhana, logo |
| Dashboard | 4 kartu ringkasan, grafik batang Pejuang per subdiv, grafik subdiv per divisi, daftar pengumuman terbaru |
| Divisi/Subdiv/Pejuang | Tabel + cari + filter + modal tambah/edit |
| Pengumuman | Daftar, form (judul, isi, tipe pop-up/banner, jadwal, target), pratinjau |
| Laporan Presensi | Filter bulan/divisi, tabel rekap, ekspor Excel |
| Laporan Upah | Tabel hitung berdasarkan presensi |

## 8. Desain UI Mobile (ringkas)
Bottom navigation: **Beranda · Presensi · Pengajuan · Slip · Profil**.
- Beranda: sapaan, tombol absen besar, banner pengumuman, pop-up saat buka.
- Presensi: kamera depan + indikator geofence ("Anda berada di area kantor").
- Pengajuan: tab Cuti, Kasbon, Mutasi, dengan status approval berbentuk timeline.
- Aksesibilitas HP kelas bawah: animasi minimal, ukuran gambar kecil, pesan error jelas.

**Gaya (Clean Minimalist / Soft UI, sejak v0.7):** latar halaman abu sangat muda `#F7F8FA`, kartu putih bersudut 20dp
dengan bayangan lembut tanpa garis, kontrol bersudut 14dp, judul halaman berupa teks besar tanpa blok warna atau ilustrasi.
Satu sumber token di `mobile/.../ui/Theme.kt` (`Hc`, `Tone`, `softShadow`).

| Peran | Warna | Catatan |
|---|---|---|
| Isi tombol utama | `#22C55E` | teks di atasnya `#052E16` (6,5:1) |
| Ikon, garis kemajuan | `#16A34A` | bukan untuk teks |
| Teks/tautan hijau | `#15803D` di putih, `#166534` di mint | ≥ 5:1 |
| Latar lembut (chip, ubin ikon, banner) | `#ECFDF3`, `#DCFCE7` | |
| Teks | `#101828`, `#475467`, placeholder `#667085` | |
| Isian | latar `#F2F4F7` tanpa garis, cincin hijau 2dp saat fokus | |
| Status | sukses `#067647`/`#ECFDF3`, peringatan `#B54708`/`#FFFAEB`, bahaya `#B42318`/`#FEF3F2`, info `#175CD3`/`#EFF8FF` | |
| Tombol merah | `#D92D20` dengan teks putih (4,8:1) | tolak, batalkan, keluar |

Huruf **Plus Jakarta Sans** (OFL, `res/font`, 4 bobot) dibundel agar sama di semua HP. Semua pasangan teks dan latar
dicek ≥ 4,5:1. Web admin masih memakai aksen `#15803D`; menyamakan aksen web dengan aplikasi menjadi tindak lanjut terpisah.

## 9. Keamanan & Privasi
- HTTPS, CORS ketat, rate limit login dan absen.
- Password argon2id/bcrypt; refresh token di-hash.
- Embedding wajah dan lokasi dianggap data pribadi: akses dibatasi, ada persetujuan di aplikasi, kebijakan retensi (mis. selfie dihapus setelah 90 hari).
- Audit log untuk aksi SDM, perubahan gaji, finalisasi payroll.
- Backup database harian + uji restore.

## 10. Konvensi
- Git: `main`, `develop`, branch `feat/...`; commit konvensional.
- Migrasi DB hanya maju lewat `golang-migrate`, tidak edit migrasi lama.
- Waktu disimpan UTC (`timestamptz`), ditampilkan WIB.
- Semua uang `NUMERIC`, tidak pernah float.
