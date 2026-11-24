-- A2F wajib per akun (D60). Super admin selalu wajib; untuk SDM super admin bisa menjadikannya
-- opsional per akun. Bawaan wajib agar akun SDM yang sudah ada tidak berubah.
ALTER TABLE users ADD COLUMN mfa_required BOOLEAN NOT NULL DEFAULT true;
