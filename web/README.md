# HRIS Pejuang: Web Admin

Web admin untuk SDM (Next.js static export, Tailwind, TanStack Query, Recharts). Acuan: `../MASTER.md`.
Web berupa file statis dan seluruh data berasal dari API Go di `../backend`.

## Menjalankan

API harus sudah berjalan, lihat `../backend/README.md`.

```bash
npm install
cp .env.example .env.local   # sekali saja
npm run dev                  # http://localhost:3000
```

Untuk presentasi pakai build produksi:

```bash
npm run build && npm start
```

Akun demo: `sdm@pejuang.id`, kata sandi = nilai `SEED_SDM_PASSWORD` di `backend/.env`.

## Struktur

```
src/app/(admin)/   halaman web admin
src/app/login/     halaman masuk
src/components/    ui/, shell/, dashboard/, employees/, announcements/, reports/
src/lib/           client HTTP, hooks data, tipe kontrak, validasi form, format WIB dan rupiah
```

Aturan penting (MASTER.md §10.1):
- Token hanya di cookie HttpOnly, tidak pernah di `localStorage`. Non-GET mengirim `X-CSRF-Token`.
- Pada 401, `api-client` mencoba refresh satu kali, bila gagal mengarahkan ke login.
- Menu disembunyikan berdasar `permissions` dari `/v1/me`, tetapi penegakan akses ada di API.
- Uang dari API berupa string; `lib/format.ts` menjumlahkannya tanpa floating point.
- Ekspor CSV menetralkan formula injection.
- Tipe API digenerate ke `src/api/schema.d.ts`. Setelah mengubah `openapi.yaml`, jalankan `npm run gen:api`.

## Perintah

| Perintah | Fungsi |
|---|---|
| `npm run dev` | Server pengembangan |
| `npm run build` | Build statis ke folder `out/` |
| `npm start` | Menyajikan `out/` di port 3000 |
| `npm run lint` | ESLint |
| `npm run gen:api` | Generate tipe dari `backend/api/openapi.yaml` |
| `npx tsc --noEmit` | Cek tipe |
