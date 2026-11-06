-- Wajah dihitung server dari selfie (D42). Setiap selfie dicatat untuk audit dan untuk menolak foto
-- yang dikirim ulang; berkasnya terenkripsi di disk, tabel ini hanya menyimpan kunci dan hash.
ALTER TABLE attendance_challenges
  ADD COLUMN liveness_action TEXT CHECK (liveness_action IN ('blink', 'turn_left', 'turn_right', 'smile'));

CREATE TABLE face_captures (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id   UUID NOT NULL REFERENCES employees(id),
  challenge_id  UUID REFERENCES attendance_challenges(id),
  attendance_id UUID REFERENCES attendances(id),
  purpose       TEXT NOT NULL CHECK (purpose IN ('clock_in', 'clock_out', 'face_enroll')),
  sha256        TEXT NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
  storage_key   TEXT NOT NULL,
  face_score    NUMERIC(5,4),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (employee_id, sha256)
);
CREATE INDEX idx_face_captures_attendance ON face_captures(attendance_id) WHERE attendance_id IS NOT NULL;
CREATE INDEX idx_face_captures_created ON face_captures(created_at);
