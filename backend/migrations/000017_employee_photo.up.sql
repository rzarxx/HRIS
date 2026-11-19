-- Foto profil Pejuang (PR 3). Berkasnya disimpan terenkripsi di SELFIE_DIR/profile/; kolom ini hanya kuncinya.
ALTER TABLE employees
    ADD COLUMN photo_key TEXT,
    ADD COLUMN photo_updated_at TIMESTAMPTZ;
