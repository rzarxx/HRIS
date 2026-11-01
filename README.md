# HRIS

HRIS Pejuang: sistem informasi SDM dengan web admin (SDM, Subdiv, Kadiv) dan aplikasi Android untuk Pejuang (presensi, pengajuan, slip upah).

## Isi repo

| Path | Isi |
|---|---|
| `web/` | Web admin Next.js. Cara menjalankan ada di [`web/README.md`](web/README.md) |
| `PRD.md` | Kebutuhan produk |
| `ROADMAP.md`, `CHECKLIST.md` | Jadwal fase dan daftar pekerjaan |
| `DESIGN.md` | Arsitektur, endpoint API, desain UI |
| `ERD.md`, `QUERY.sql` | Skema database PostgreSQL dan query penting |
| `FLOWCHART.md` | Alur login, presensi, approval, payroll, pengumuman |
| `MCP.md` | Panduan pengembangan dengan asisten AI |

## Mulai cepat

```bash
cd web
npm install
cp .env.example .env.local
npm run dev
```
