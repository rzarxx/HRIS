-- Fondasi Fase 1: MFA TOTP dan aktivasi lewat undangan (MASTER.md Lampiran A2).
CREATE TABLE user_mfa (
  user_id         UUID PRIMARY KEY REFERENCES users(id),
  secret_enc      BYTEA NOT NULL, -- AES-256-GCM, nonce di depan
  enabled_at      TIMESTAMPTZ,
  recovery_hashes TEXT[] NOT NULL DEFAULT '{}',
  last_used_step  BIGINT NOT NULL DEFAULT 0 -- mencegah kode TOTP dipakai ulang
);

CREATE TABLE user_invites (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES users(id),
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at    TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_user_invites_user ON user_invites(user_id) WHERE used_at IS NULL;
