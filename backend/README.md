# HRIS Pejuang: Backend API

API Go (chi, pgx, sqlc, golang-migrate) dengan PostgreSQL. Acuan: `../MASTER.md`. Kontrak: `api/openapi.yaml`.

## Prasyarat
- Go 1.27+
- PostgreSQL 16+ berjalan di `localhost:5432`
- [sqlc](https://sqlc.dev) hanya bila mengubah file di `queries/` atau `migrations/`

## Menyiapkan (sekali)

Dari folder root repo. Skrip menanyakan kata sandi superuser `postgres`, lalu membuat database `hris_dev`,
role `hris_migrate`, `hris_app`, `hris_ro`, dan menulis `backend/.env` berisi kata sandi acak (tidak masuk Git).

```bash
powershell -ExecutionPolicy Bypass -File backend\scripts\dev-db-setup.ps1
```

Lalu dari folder `backend`:

```bash
go run ./cmd/migrate up
```
```bash
go run ./cmd/seed
```

Akun demo: `sdm@pejuang.id`, kata sandi = nilai `SEED_SDM_PASSWORD` di `backend/.env`.

## Menjalankan

```bash
go run ./cmd/api
```

API berjalan di `http://localhost:8080`. Cek dengan `GET /healthz` dan `GET /readyz`.

Saat `DEV_SIMULATE_ATTENDANCE=true`, API mengisi presensi dummy sampai hari ini setiap 5 menit.
Presensi nyata dari aplikasi HP menyusul di Fase 2-3.

Mengembalikan data dummy ke kondisi awal (sebelum demo, misalnya):

```bash
go run ./cmd/seed --reset
```

## Struktur

```
cmd/api, cmd/migrate, cmd/seed    program
api/openapi.yaml                  kontrak API, bisa diimpor ke Postman
migrations/                       migrasi, hanya maju
queries/                          SQL untuk sqlc
internal/store/                   hasil generate sqlc, jangan diedit manual
internal/permission/              peta role ke permission (satu sumber)
internal/auth/                    login, JWT, sesi, CSRF
internal/server/                  tabel route + middleware
internal/org, employee, announcement, dashboard, report   modul bisnis
internal/devsim/                  seed dan simulasi presensi, khusus pengembangan
```

Lapisan: handler (HTTP, decode) → service (aturan, scope, transaksi) → store (sqlc).

## Perintah

| Perintah | Fungsi |
|---|---|
| `go test ./...` | Tes unit dan matriks permission |
| `go vet ./...` | Pemeriksaan statis |
| `sqlc generate` | Generate ulang `internal/store` setelah mengubah query atau migrasi |

## MFA saat pengembangan

`MFA_REQUIRED=false` di `.env` lokal agar akun demo bisa masuk tanpa authenticator. Nilai bawaannya `true`,
dan API menolak start bila `false` di luar `APP_ENV=development`.

## Yang belum ada
Staging HTTPS dan backup (menunggu keputusan hosting), rate limit berbasis Redis, audit log lengkap,
serta tes integrasi dengan database asli (scope 404, lockout, rotasi token, aktivasi).
