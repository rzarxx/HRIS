-- Tautan reset kata sandi lewat email (D30, MASTER.md §5.2.1). Yang disimpan hanya SHA-256 token;
-- token asli hanya ada di email. Satu tautan sekali pakai, berlaku 1 jam, dan tautan baru membatalkan
-- yang lama.
CREATE TABLE password_reset_tokens (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES users(id),
  token_hash TEXT NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  expires_at TIMESTAMPTZ NOT NULL,
  used_at    TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_password_reset_tokens_user ON password_reset_tokens(user_id) WHERE used_at IS NULL;
