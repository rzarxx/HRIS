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

Play Integrity bersifat opsional dan belum teruji dengan aplikasi Android nyata. Untuk pengujian,
atur `PLAY_INTEGRITY_PACKAGE` ke package aplikasi dan `PLAY_INTEGRITY_CREDENTIALS` ke path file
service account Google yang memiliki akses Play Integrity. Kedua nilai harus diisi bersama dan tidak
boleh digabung dengan `DEV_ALLOW_UNVERIFIED_ATTENDANCE=true`. Konfigurasi ini ditolak di luar
development sampai pengikatan kriptografis instalasi perangkat selesai. Tanpa konfigurasi itu, presensi dan
pendaftaran wajah tetap ditolak di luar mode development yang secara eksplisit mengizinkan bypass.
Jangan commit file kredensial. Gangguan layanan Google saat ini menyebabkan penolakan tertutup.

Retensi selfie dijalankan harian dengan environment yang sama dengan API dan `SELFIE_DIR` yang
menunjuk penyimpanan sebenarnya. Di lokal, dari direktori `backend`:

```bash
go run ./cmd/purge-selfies
```

Jalankan migrasi terlebih dahulu. Perintah menghapus berkas setelah 90 hari, mempertahankan hash
untuk menolak replay, dan dapat dijalankan ulang setelah gagal. Di server development, `deploy.sh`
membangun `hris-purge-selfies` dan memasang timer systemd harian (lihat `deploy/README.md`).

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
| `go test ./...` | Tes unit, matriks permission, dan kontrak |
| `HRIS_TEST_DATABASE_URL=... go test ./internal/integration` | Tes integrasi dengan PostgreSQL asli |
| `go vet ./...` | Pemeriksaan statis |
| `sqlc generate` | Generate ulang `internal/store` setelah mengubah query atau migrasi |

## Tes integrasi

`internal/integration` menguji API lengkap sebagai role `hris_app`: login, lockout, rotasi token, MFA, aktivasi, scope 404,
audit append-only, master data, dan laporan upah. Tes dilewati bila `HRIS_TEST_DATABASE_URL` kosong, dan selalu dijalankan di CI.

**Tes mengosongkan database yang dituju.** Karena itu nama databasenya wajib mengandung kata `test`, dan role superuser diperlukan.

## MFA saat pengembangan

`MFA_REQUIRED=false` di `.env` lokal agar akun demo bisa masuk tanpa authenticator. Nilai bawaannya `true`,
dan API menolak start bila `false` di luar `APP_ENV=development`.

## Yang belum ada
Staging HTTPS dan backup (menunggu keputusan hosting), rate limit berbasis Redis, dan audit log lengkap.
