# ERD: HRIS Pejuang

Database: PostgreSQL + PostGIS + pgvector. DDL lengkap ada di `QUERY.sql`.

## Diagram

```mermaid
erDiagram
    DIVISIONS ||--o{ SUBDIVISIONS : has
    SUBDIVISIONS ||--o{ EMPLOYEES : has
    EMPLOYEES ||--|| USERS : login
    EMPLOYEES ||--o{ DEVICES : registers
    EMPLOYEES ||--o{ FACE_TEMPLATES : has
    EMPLOYEES ||--o{ ATTENDANCES : records
    EMPLOYEES }o--o{ OFFICE_LOCATIONS : allowed_at
    SHIFTS ||--o{ ATTENDANCES : applies
    OFFICE_LOCATIONS ||--o{ ATTENDANCES : at
    EMPLOYEES ||--o{ LEAVE_REQUESTS : submits
    LEAVE_TYPES ||--o{ LEAVE_REQUESTS : typed
    EMPLOYEES ||--o{ CASH_ADVANCES : submits
    EMPLOYEES ||--o{ TRANSFER_REQUESTS : subject
    SUBDIVISIONS ||--o{ TRANSFER_REQUESTS : from_to
    PAYROLL_PERIODS ||--o{ PAYROLLS : contains
    EMPLOYEES ||--o{ PAYROLLS : receives
    PAYROLLS ||--o{ PAYROLL_ITEMS : details
    EMPLOYEES ||--o{ WARNINGS : receives
    ANNOUNCEMENTS ||--o{ ANNOUNCEMENT_READS : read_by
    EMPLOYEES ||--o{ ANNOUNCEMENT_READS : reads
    EMPLOYEES ||--o{ NOTIFICATIONS : gets
    USERS ||--o{ AUDIT_LOGS : acts

    DIVISIONS {
        uuid id PK
        text code
        text name
    }
    SUBDIVISIONS {
        uuid id PK
        uuid division_id FK
        text name
        uuid head_employee_id FK
    }
    EMPLOYEES {
        uuid id PK
        text nip
        text full_name
        uuid subdivision_id FK
        text position
        numeric base_salary
        text status
    }
    USERS {
        uuid id PK
        uuid employee_id FK
        text email
        text password_hash
        enum role
    }
    DEVICES {
        uuid id PK
        uuid employee_id FK
        text device_hash
        text fcm_token
        text status
    }
    OFFICE_LOCATIONS {
        uuid id PK
        text name
        geography center
        int radius_m
        geography polygon
    }
    ATTENDANCES {
        uuid id PK
        uuid employee_id FK
        date work_date
        timestamptz clock_in_at
        timestamptz clock_out_at
        numeric face_score
        int risk_score
        text status
    }
    APPROVALS {
        uuid id PK
        text request_type
        uuid request_id
        int step_order
        text approver_role
        text status
    }
    PAYROLLS {
        uuid id PK
        uuid period_id FK
        uuid employee_id FK
        numeric gross
        numeric net
    }
```

`APPROVALS` bersifat polimorfik (`request_type` + `request_id`) sehingga tidak digambar dengan garis relasi. Dipakai oleh `leave_requests`, `cash_advances`, `transfer_requests`.

## Daftar Tabel

| Tabel | Fungsi |
|---|---|
| divisions, subdivisions | Struktur organisasi |
| employees | Data Pejuang |
| users | Akun login + role |
| devices | Device binding, token FCM |
| face_templates | Embedding wajah (pgvector) |
| office_locations, employee_locations | Geofence, lokasi yang diizinkan per Pejuang |
| shifts | Jam kerja & toleransi telat |
| attendances | Presensi harian, skor risiko |
| leave_types, leave_requests | Cuti & izin |
| cash_advances | Kasbon |
| transfer_requests | Mutasi |
| approvals | Mesin approval generik |
| payroll_periods, payrolls, payroll_items | Upah & komponen |
| warnings | Surat Peringatan |
| announcements, announcement_reads | Pengumuman |
| notifications | Riwayat notifikasi |
| audit_logs | Jejak aksi |

## Aturan Data
- `attendances`: unik `(employee_id, work_date)`.
- `payrolls`: unik `(period_id, employee_id)`; periode `final` tidak boleh diubah.
- Ganti perangkat: `devices.status` = `pending` sampai disetujui SDM.
- Semua tabel utama punya `created_at`, `updated_at`; hapus data memakai soft delete (`deleted_at`) untuk employees.
- Mutasi disetujui penuh → job memperbarui `employees.subdivision_id` pada `effective_date`.
