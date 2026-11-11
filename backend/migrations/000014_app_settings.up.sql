-- Pengaturan aplikasi sederhana berbentuk kunci-nilai, diatur super admin (settings:manage).
-- Kunci pertama: footer_template untuk footer web admin dan aplikasi; tanpa baris berarti teks bawaan.
CREATE TABLE app_settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,
  updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
