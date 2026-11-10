// Validasi form di sisi klien untuk pengalaman pengguna. Penegakan tetap di API Go.
import { z } from "zod";

const required = (label: string) => z.string({ error: `${label} wajib diisi.` }).trim().min(1, `${label} wajib diisi.`);
const optionalText = z.string().trim().nullable().optional();
const code = (max: number) =>
  required("Kode")
    .max(max, `Kode maksimal ${max} karakter.`)
    .regex(/^[A-Za-z0-9-]+$/, "Kode hanya huruf, angka, dan tanda minus.");

export const loginSchema = z.object({
  email: required("Email").email("Format email tidak valid."),
  password: required("Kata sandi"),
});

export const divisionSchema = z.object({
  code: code(10),
  name: required("Nama divisi").max(80, "Nama maksimal 80 karakter."),
  head_employee_id: z.string().uuid().nullable().optional(),
});

export const subdivisionSchema = z.object({
  division_id: required("Divisi").uuid("Pilih divisi."),
  code: code(12),
  name: required("Nama subdiv").max(80, "Nama maksimal 80 karakter."),
  head_employee_id: z.string().uuid().nullable().optional(),
});

const blankOr = (test: (v: string) => boolean) => (v: string | null | undefined) => !v || test(v);

export const employeeSchema = z.object({
  nip: optionalText.refine(blankOr((v) => /^[A-Za-z0-9-]{4,20}$/.test(v)), "NIPY 4-20 karakter huruf/angka."),
  full_name: required("Nama lengkap").min(3, "Nama minimal 3 karakter.").max(100, "Nama maksimal 100 karakter."),
  email: required("Email").email("Format email tidak valid."),
  phone: optionalText.refine(blankOr((v) => /^(\+62|0)8[0-9]{7,12}$/.test(v)), "Nomor HP tidak valid (contoh 081234567890)."),
  subdivision_id: required("Subdiv").uuid("Pilih subdiv."),
  position: optionalText,
  join_date: optionalText.refine(blankOr((v) => /^\d{4}-\d{2}-\d{2}$/.test(v)), "Tanggal tidak valid."),
  base_salary: required("Ubar pokok").regex(/^\d+(\.\d{1,2})?$/, "Ubar pokok harus angka.").optional(),
  status: z.enum(["active", "inactive"]),
  default_shift_id: z.string().uuid().nullable().optional(),
});

export const announcementSchema = z
  .object({
    title: required("Judul").max(120, "Judul maksimal 120 karakter."),
    body: required("Isi").max(2000, "Isi maksimal 2000 karakter."),
    type: z.enum(["popup", "banner"], { error: "Pilih tipe." }),
    division_id: z.string().uuid().nullable().optional(),
    starts_at: required("Mulai tayang").datetime({ error: "Tanggal mulai tidak valid." }),
    ends_at: z.string().datetime({ error: "Tanggal berakhir tidak valid." }).nullable().optional(),
    is_active: z.boolean(),
  })
  .refine((v) => !v.ends_at || v.ends_at > v.starts_at, { path: ["ends_at"], message: "Berakhir harus setelah mulai tayang." });

export type DivisionInput = z.input<typeof divisionSchema>;
export type SubdivisionInput = z.input<typeof subdivisionSchema>;
export type EmployeeInput = z.input<typeof employeeSchema>;
export type AnnouncementInput = z.input<typeof announcementSchema>;

// Koma desimal diterima karena lazim di Indonesia.
export const parseCoordinate = (v: string) => Number(v.trim().replace(",", "."));

const coordinate = (label: string, max: number) =>
  required(label).refine((v) => Math.abs(parseCoordinate(v)) <= max, `${label} harus antara -${max} dan ${max}.`);

// Satu titik per baris: "lintang, bujur". Mengembalikan null bila kosong atau formatnya salah.
export function parsePolygon(text: string) {
  const lines = text.split("\n").map((l) => l.trim()).filter(Boolean);
  if (lines.length === 0) return null;
  const points = lines.map((line) => {
    const [latitude, longitude, extra] = line.split(/[,\s]+/).map(Number);
    return { latitude, longitude, valid: extra === undefined && Math.abs(latitude) <= 90 && Math.abs(longitude) <= 180 };
  });
  if (points.some((p) => !p.valid)) return null;
  return points.map(({ latitude, longitude }) => ({ latitude, longitude }));
}

export const locationSchema = z.object({
  name: required("Nama lokasi").max(120, "Nama maksimal 120 karakter."),
  latitude: coordinate("Lintang", 90),
  longitude: coordinate("Bujur", 180),
  radius_m: required("Radius").refine((v) => Number.isInteger(Number(v)) && Number(v) >= 10 && Number(v) <= 5000, "Radius harus antara 10 dan 5000 meter."),
  polygon: z.string().refine((v) => {
    if (!v.trim()) return true;
    const points = parsePolygon(v);
    return points !== null && points.length >= 3 && points.length <= 100;
  }, "Isi 3 sampai 100 titik, satu per baris: lintang, bujur."),
  is_active: z.boolean(),
});
export type LocationForm = z.infer<typeof locationSchema>;

const clock = /^([01]\d|2[0-3]):[0-5]\d$/;

export const shiftSchema = z
  .object({
    name: required("Nama shift").max(80, "Nama maksimal 80 karakter."),
    start_time: z.string().regex(clock, "Isi jam masuk."),
    end_time: z.string().regex(clock, "Isi jam pulang."),
    late_tolerance_minutes: required("Toleransi").refine((v) => Number.isInteger(Number(v)) && Number(v) >= 0 && Number(v) <= 240, "Toleransi harus 0 sampai 240 menit."),
    work_days: z.array(z.number()).min(1, "Pilih minimal satu hari kerja."),
  })
  .refine((v) => v.start_time !== v.end_time, { path: ["end_time"], message: "Jam pulang tidak boleh sama dengan jam masuk." });
export type ShiftForm = z.infer<typeof shiftSchema>;

export const holidaySchema = z.object({
  date: required("Tanggal"),
  name: required("Keterangan").max(120, "Keterangan maksimal 120 karakter."),
});
export type HolidayInput = z.infer<typeof holidaySchema>;

export const workArrangementSchema = z
  .object({
    mode: z.enum(["wfa", "hybrid"], { error: "Pilih mode." }),
    flexible_hours: z.boolean(),
    starts_on: required("Tanggal mulai"),
    ends_on: z.string().nullable(),
    weekdays: z.array(z.number()),
    reason: required("Alasan").max(500, "Alasan maksimal 500 karakter."),
  })
  .refine((v) => !v.ends_on || v.ends_on >= v.starts_on, { path: ["ends_on"], message: "Tanggal selesai tidak boleh sebelum tanggal mulai." });

/** Mengembalikan pesan error per field atau null bila valid. */
export function validate(schema: z.ZodType, values: unknown): Record<string, string> | null {
  const r = schema.safeParse(values);
  if (r.success) return null;
  const fields: Record<string, string> = {};
  for (const issue of r.error.issues) fields[issue.path.join(".")] ??= issue.message;
  return fields;
}
