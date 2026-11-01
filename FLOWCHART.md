# FLOWCHART: HRIS Pejuang

Semua diagram memakai Mermaid (tampil otomatis di GitHub, VS Code, Notion).

## 1. Login & Role

```mermaid
flowchart TD
    A[Buka aplikasi / web] --> B[Input email + password]
    B --> C{Kredensial valid?}
    C -- Tidak --> B
    C -- Ya --> D[API terbitkan JWT + refresh token]
    D --> E{Role?}
    E -- Pejuang --> F[Dashboard Pejuang]
    E -- Subdiv/Kadiv --> G[Dashboard Pejuang + antrian approval]
    E -- SDM --> H[Dashboard SDM web]
```

## 2. Presensi (Clock In)

```mermaid
flowchart TD
    A[Pejuang tekan Absen] --> B[HP: cek mock location, root, app fake GPS]
    B --> C{Terdeteksi?}
    C -- Ya --> X1[Tolak + catat log]
    C -- Tidak --> D[HP: ambil lokasi + selfie + liveness + embedding]
    D --> E{Liveness lolos?}
    E -- Tidak --> D2[Coba ulang, maks 3x]
    D2 --> X2[Status review]
    E -- Ya --> F[Kirim: lokasi, embedding, token Play Integrity, device id]
    F --> G{Server: device terdaftar & approved?}
    G -- Tidak --> X3[Tolak: perangkat belum disetujui]
    G -- Ya --> H{Integrity valid?}
    H -- Tidak --> X1
    H -- Ya --> I{Dalam geofence?}
    I -- Tidak --> X4[Tolak: di luar area]
    I -- Ya --> J{Wajah cocok?}
    J -- Tidak --> X5[Tolak atau review]
    J -- Ya --> K[Hitung skor risiko]
    K --> L{Skor?}
    L -- "< 30" --> M[Simpan present / late]
    L -- "30-59" --> N[Simpan status review]
    L -- ">= 60" --> X1
    M --> O[Push notifikasi sukses]
    N --> O2[Notifikasi: menunggu review SDM]
```

## 3. Mesin Approval Generik

```mermaid
flowchart TD
    A[Pejuang kirim pengajuan] --> B[Buat record request + baris approvals sesuai alur]
    B --> C[Step 1: notifikasi ke approver]
    C --> D{Keputusan}
    D -- Reject --> R[Status rejected, notifikasi ke Pejuang]
    D -- Approve --> E{Ada step berikutnya?}
    E -- Ya --> F[Step berikutnya aktif + notifikasi]
    F --> D
    E -- Tidak --> G[Status approved]
    G --> H[Aksi lanjutan sesuai jenis]
    H --> I[Notifikasi ke Pejuang]
```

## 4. Alur per Jenis Pengajuan

```mermaid
flowchart LR
    subgraph Cuti
      C1[Pejuang] --> C2[Subdiv] --> C3[SDM]
    end
    subgraph Kasbon
      K1[Pejuang] --> K2[Subdiv] --> K3[Kadiv] --> K4[Potongan upah dijadwalkan]
    end
    subgraph Mutasi
      M1[Pejuang] --> M2[Subdiv asal] --> M3[Kadiv] --> M4[SDM] --> M5[Subdiv employee diperbarui di tanggal efektif]
    end
```

Alur di atas adalah asumsi awal; konfirmasi di PRD bagian 12.

## 5. Ganti Perangkat

```mermaid
flowchart TD
    A[Login di HP baru] --> B[Perangkat tercatat status pending]
    B --> C[Presensi diblokir]
    C --> D[SDM tinjau di web admin]
    D --> E{Setujui?}
    E -- Ya --> F[HP baru approved, HP lama revoked]
    E -- Tidak --> G[Tetap diblokir]
```

## 6. Payroll Bulanan

```mermaid
flowchart TD
    A[SDM buat periode] --> B[Generate: tarik rekap presensi]
    B --> C[Hitung komponen: gaji pokok, lembur, potongan telat/alpha, potongan kasbon]
    C --> D[Draft payroll, SDM tinjau & koreksi]
    D --> E{Sudah benar?}
    E -- Tidak --> C
    E -- Ya --> F[Finalisasi: periode dikunci]
    F --> G[Buat slip PDF]
    G --> H[Notifikasi ke Pejuang, slip bisa diunduh]
```

## 7. Pengumuman

```mermaid
flowchart TD
    A[SDM buat pengumuman] --> B[Pilih tipe: pop-up / banner, target, jadwal]
    B --> C[Simpan]
    C --> D[Pada waktu tayang: tampil di app]
    D --> E[Pejuang buka app]
    E --> F{Tipe?}
    F -- Pop-up --> G[Modal tampil sekali sampai ditandai dibaca]
    F -- Banner --> H[Banner di beranda sampai berakhir]
```

## 8. Alur Demo Jumat

```mermaid
flowchart LR
    A[Login SDM] --> B[Dashboard grafik]
    B --> C[Tambah Pejuang baru]
    C --> D[Grafik berubah]
    D --> E[Buat pengumuman]
    E --> F[Lihat rekap presensi]
    F --> G[Laporan upah]
    G --> H[Slide roadmap]
```
