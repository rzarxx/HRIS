-- Ganti email oleh Pejuang sendiri (D59): email baru berlaku setelah tautan konfirmasi di email baru
-- dibuka. Yang disimpan hanya SHA-256 token; permintaan baru membatalkan yang lama.
CREATE TABLE email_change_requests (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES users(id),
  new_email  TEXT NOT NULL CHECK (length(new_email) BETWEEN 3 AND 254),
  token_hash TEXT NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
  expires_at TIMESTAMPTZ NOT NULL,
  used_at    TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_email_change_requests_user ON email_change_requests(user_id, created_at);
