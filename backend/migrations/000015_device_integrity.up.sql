-- Fase 2: jalur verifikasi integritas perangkat dan pengikatan kunci instalasi (MASTER.md §5.3, §5.4.1).
CREATE TYPE device_integrity_tier AS ENUM ('play', 'attestation', 'manual', 'kiosk');

ALTER TABLE devices
  ADD COLUMN integrity_tier        device_integrity_tier NOT NULL DEFAULT 'play',
  ADD COLUMN manual_approved_until TIMESTAMPTZ,
  -- Kunci publik ECDSA P-256 (PKIX DER) milik instalasi. Diisi sekali, tidak pernah ditimpa.
  ADD COLUMN public_key            BYTEA,
  ADD COLUMN key_bound_at          TIMESTAMPTZ,
  ADD COLUMN tier_changed_by       UUID REFERENCES users(id),
  ADD COLUMN tier_changed_at       TIMESTAMPTZ,
  ADD CONSTRAINT chk_devices_manual_until CHECK (integrity_tier <> 'manual' OR manual_approved_until IS NOT NULL),
  ADD CONSTRAINT chk_devices_public_key_len CHECK (public_key IS NULL OR octet_length(public_key) BETWEEN 64 AND 200);
