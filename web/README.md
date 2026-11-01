# HRIS Pejuang — Web Admin

Web admin untuk SDM (Next.js 15 + Tailwind 4 + TanStack Query + Recharts). Dokumen produk ada di folder induk: `PRD.md`, `DESIGN.md`, `ERD.md`, `ROADMAP.md`.

## Menjalankan

```bash
npm install
cp .env.example .env.local   # sekali saja
npm run dev                  # http://localhost:3000
```

Untuk presentasi pakai build produksi (lebih cepat, tidak ada overlay error):

```bash
npm run build && npm start
```

Akun demo (dibuat otomatis dari seed): `sdm@pejuang.id` / nilai `SEED_SDM_PASSWORD` di `.env.local` (bawaan `pejuang123`).

Data dummy disimpan di `.data/db.json`. Untuk mengembalikan ke kondisi awal: menu **Pengaturan → Reset data dummy**, atau hapus folder `.data/`.

## Arsitektur

```
src/
├─ app/(admin)/        halaman web admin (dashboard, master data, presensi, payroll, pengumuman, ...)
├─ app/login/          halaman login
├─ app/api/v1/         API SEMENTARA — meniru kontrak endpoint Go di DESIGN.md §5
├─ mock-api/           penyimpanan JSON, seed, simulasi presensi, query laporan (padanan QUERY.sql)
├─ components/         UI (ui/, shell/, dashboard/, employees/, announcements/, reports/)
└─ lib/                tipe kontrak API, client HTTP, hooks data, skema validasi (zod), format WIB/rupiah
```

**Halaman tidak pernah memanggil `mock-api/` langsung**, semuanya lewat `lib/api-client.ts` ke `/v1/...`.
Saat backend Go siap:

1. Isi `NEXT_PUBLIC_API_URL=https://api.domain-anda` lalu build ulang.
2. Backend Go mengikuti bentuk JSON di `lib/types.ts` (snake_case, `{ data }`, `{ data, meta }` untuk paginasi,
   error `{ error: { code, message, fields? } }`).
3. Hapus `src/app/api/` dan `src/mock-api/`.

Aturan bisnis yang sementara ada di `mock-api/` (wajib dipindah ke Go):
- Rumus upah sementara: `queries.ts → PAYROLL_RULES` (alpha = gaji/26 per hari, telat Rp5.000/kejadian).
- Presensi masih simulasi (`attendance.ts`), termasuk pembuatan baris `absent` untuk alpha.
- Tanggal kerja memakai WIB (`Asia/Jakarta`), bukan `current_date` server UTC.

## Status fitur (Fase 0)

| Modul | Status |
|---|---|
| Login + sesi JWT (cookie httpOnly) | ✅ |
| Dashboard: kartu ringkasan, grafik Pejuang/subdiv & subdiv/divisi, pengumuman terbaru | ✅ |
| Master Divisi, Subdiv, Pejuang (CRUD, cari, filter, paginasi, validasi) | ✅ |
| Pengumuman (CRUD, jadwal, target divisi, pratinjau pop-up/banner) | ✅ |
| Rekap presensi & laporan upah (filter bulan/divisi, ekspor CSV untuk Excel) | ✅ (data simulasi) |
| Lokasi kantor, shift, review absen | Fase 2 |
| Cuti, kasbon, mutasi | Fase 4 |
| Surat peringatan, slip payroll | Fase 5 |

## Perintah

| Perintah | Fungsi |
|---|---|
| `npm run dev` | Server pengembangan |
| `npm run build` | Build produksi (termasuk cek tipe) |
| `npm run lint` | ESLint |
| `npx tsc --noEmit` | Cek tipe saja |
