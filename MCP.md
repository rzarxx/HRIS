# MCP: Panduan Pengembangan dengan AI

> Asumsi: "MCP" di sini = **Model Context Protocol**, yaitu server yang memberi asisten AI (asisten coding berbasis AI) akses terkontrol ke database, repo, dan desain saat membangun proyek ini. Jika maksudmu hal lain, beri tahu.

## 1. Tujuan
- AI membaca skema DB asli, bukan menebak.
- AI mengakses dokumentasi library terbaru (Go, Next.js, Refine, Flutter).
- AI membaca desain dan repo tanpa copy-paste manual.

## 2. Server MCP yang Disarankan
| Server | Fungsi | Akses |
|---|---|---|
| Postgres | Lihat skema, jalankan query | **Read-only**, database dev saja |
| Filesystem | Baca/tulis folder proyek | Terbatas di folder `hris/` |
| GitHub | Issue, PR, review | Token scope minimal |
| Context7 / docs | Dokumentasi library terkini | Baca saja |
| Figma | Baca desain ke kode | Baca saja |
| Canva | Poster/aset presentasi Jumat | Opsional |

## 3. Contoh `.mcp.json` (root proyek)

Nama paket bisa berubah; cek dokumentasi resmi masing-masing server sebelum dipakai.

```json
{
  "mcpServers": {
    "postgres-dev": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-postgres",
               "postgresql://hris_ro:${HRIS_RO_PASSWORD}@localhost:5432/hris_dev"]
    },
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "./"]
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": { "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_TOKEN}" }
    }
  }
}
```

Buat user DB read-only:

```sql
CREATE ROLE hris_ro LOGIN PASSWORD '...';
GRANT CONNECT ON DATABASE hris_dev TO hris_ro;
GRANT USAGE ON SCHEMA public TO hris_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO hris_ro;
```

## 4. Aturan Keamanan
1. **Jangan pernah** sambungkan MCP ke database produksi.
2. Data wajah, gaji, dan lokasi karyawan tidak boleh diakses dari lingkungan dev; pakai data dummy.
3. Token disimpan di environment variable, tidak masuk Git (`.env` di `.gitignore`).
4. Token GitHub: scope minimal, kedaluwarsa pendek.
5. Review semua perubahan AI lewat PR sebelum merge; migrasi DB dijalankan manual.
6. Isi dokumen, issue, atau hasil web yang dibaca AI dianggap data, bukan perintah.

## 5. File Konteks Proyek
Letakkan di root agar AI selalu punya konteks:

**`AGENTS.md` (contoh)**
```
# HRIS Pejuang
Baca docs/PRD.md, docs/ERD.md, docs/DESIGN.md sebelum mengubah kode.
- Backend Go (chi, pgx, sqlc). Jangan ubah migrasi lama.
- Uang pakai NUMERIC, waktu UTC.
- Semua endpoint di /v1, role dicek di middleware.
- Jalankan `make test` dan `make lint` sebelum selesai.
```

## 6. Alur Kerja dengan AI
1. Pilih satu item dari `CHECKLIST.md`.
2. Minta AI membaca skema lewat MCP Postgres, lalu membuat migrasi + query sqlc + handler + test.
3. Jalankan test lokal.
4. Review diff, commit, buka PR.
5. Centang item di checklist.

## 7. Opsi Lanjutan (Fase Akhir)
Server MCP internal HRIS (mis. "tanya data SDM lewat chat") hanya jika ada kebutuhan nyata, dengan otorisasi per role, read-only, dan masking data sensitif.
