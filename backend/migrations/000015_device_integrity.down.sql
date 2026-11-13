ALTER TABLE devices
  DROP CONSTRAINT IF EXISTS chk_devices_public_key_len,
  DROP CONSTRAINT IF EXISTS chk_devices_manual_until,
  DROP COLUMN IF EXISTS tier_changed_at,
  DROP COLUMN IF EXISTS tier_changed_by,
  DROP COLUMN IF EXISTS key_bound_at,
  DROP COLUMN IF EXISTS public_key,
  DROP COLUMN IF EXISTS manual_approved_until,
  DROP COLUMN IF EXISTS integrity_tier;

DROP TYPE IF EXISTS device_integrity_tier;
