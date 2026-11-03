-- Fase 2: lokasi kantor dan hari libur. Geofence dihitung di Go, tanpa PostGIS (D22).
CREATE TABLE office_locations (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL,
  latitude   NUMERIC(9,6) NOT NULL CHECK (latitude BETWEEN -90 AND 90),
  longitude  NUMERIC(9,6) NOT NULL CHECK (longitude BETWEEN -180 AND 180),
  radius_m   INT NOT NULL DEFAULT 100 CHECK (radius_m BETWEEN 10 AND 5000),
  polygon    JSONB, -- daftar {latitude, longitude}; bila ada, menggantikan radius
  is_active  BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE employee_locations (
  employee_id UUID NOT NULL REFERENCES employees(id),
  location_id UUID NOT NULL REFERENCES office_locations(id),
  PRIMARY KEY (employee_id, location_id)
);
CREATE INDEX idx_employee_locations_location ON employee_locations(location_id);

CREATE TABLE holidays (
  date DATE PRIMARY KEY,
  name TEXT NOT NULL
);
