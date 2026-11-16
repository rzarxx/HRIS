// Validasi form di sisi klien untuk pengalaman pengguna. Penegakan tetap di API Go.
import { z } from "zod";

const required = (label: string) => z.string({ error: `${label} wajib diisi.` }).trim().min(1, `${label} wajib diisi.`);
const optionalText = z.string().trim().nullable().optional();
const code = (max: number) =>
  required("Kode")
    .max(max, `Kode maksimal ${max} karakter.`)
    .regex(/^[A-Za-z0-9-]+$/, "Kode hanya huruf, angka, dan tanda minus.");

export const loginSchema = z.object({
  identifier: required("NIPY, email, atau nomor WA"),
  password: required("Kata sandi"),
});

export const branchSchema = z.object({
  code: code(20).min(2, "Kode minimal 2 karakter."),
  name: required("Nama cabang").max(80, "Nama maksimal 80 karakter."),
});

export const divisionSchema = z.object({
  branch_id: required("Cabang").uuid("Pilih cabang."),
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

// Nilai resmi formulir data Pejuang (MASTER.md §6.2, D53).
export const GENDERS = { L: "Laki-laki", P: "Perempuan" } as const;
export const MARITAL_STATUSES = { belum_menikah: "Belum menikah", menikah: "Menikah", cerai_hidup: "Cerai hidup", cerai_mati: "Cerai mati" } as const;
export const CHILD_RELATIONS = { kandung: "Kandung", angkat: "Angkat", tiri: "Tiri" } as const;
export const EDUCATION_LEVELS = {
  belum_sekolah: "Belum sekolah",
  paud: "PAUD",
  tk_ra: "TK/RA",
  sd_mi: "SD/MI",
  smp_mts: "SMP/MTs",
  sma_sederajat: "SMA/Sederajat",
  perguruan_tinggi: "Perguruan tinggi",
  pondok_pesantren: "Pondok pesantren",
} as const;

const enumKeys = <T extends Record<string, string>>(o: T) => Object.keys(o) as [keyof T & string, ...(keyof T & string)[]];
const isoDate = (v: string) => /^\d{4}-\d{2}-\d{2}$/.test(v);

export const childSchema = z.object({
  child_order: z.number({ error: "Isi anak ke-." }).int().min(1, "Anak ke- 1 sampai 30.").max(30, "Anak ke- 1 sampai 30."),
  full_name: required("Nama anak").min(2, "Nama anak minimal 2 karakter.").max(100, "Nama anak maksimal 100 karakter."),
  gender: z.enum(enumKeys(GENDERS)).nullable().optional(),
  birth_place: optionalText,
  birth_date: optionalText.refine(blankOr(isoDate), "Tanggal lahir tidak valid."),
  relation: z.enum(enumKeys(CHILD_RELATIONS)).nullable().optional(),
  education: z.enum(enumKeys(EDUCATION_LEVELS)).nullable().optional(),
  school_name: optionalText,
});

export const employeeSchema = z.object({
  nip: optionalText.refine(blankOr((v) => /^[A-Za-z0-9.-]{4,20}$/.test(v)), "NIPY 4-20 karakter huruf, angka, titik, atau minus."),
  full_name: required("Nama lengkap").min(3, "Nama minimal 3 karakter.").max(100, "Nama maksimal 100 karakter."),
  email: optionalText.refine(blankOr((v) => z.string().email().safeParse(v).success), "Format email tidak valid."),
  phone: optionalText.refine(blankOr((v) => /^(\+?62|0)?8[0-9]{7,12}$/.test(v.replace(/[\s-]/g, ""))), "Nomor WA tidak valid (contoh 081234567890)."),
  gender: z.enum(enumKeys(GENDERS)).nullable().optional(),
  marital_status: z.enum(enumKeys(MARITAL_STATUSES)).nullable().optional(),
  has_children: z.boolean().nullable().optional(),
  children_count: z.number().int().min(0, "Jumlah anak 0 sampai 30.").max(30, "Jumlah anak 0 sampai 30.").nullable().optional(),
  children: z.array(childSchema).max(10).optional(),
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
    min_read_seconds: z
      .number({ error: "Isi jeda baca dalam detik." })
      .int("Jeda baca berupa bilangan bulat.")
      .min(0, "Jeda baca 0 sampai 120 detik.")
      .max(120, "Jeda baca 0 sampai 120 detik."),
  })
  .refine((v) => !v.ends_at || v.ends_at > v.starts_at, { path: ["ends_at"], message: "Berakhir harus setelah mulai tayang." });

export type BranchInput = z.input<typeof branchSchema>;
export type DivisionInput = z.input<typeof divisionSchema>;
export type ChildInput = z.input<typeof childSchema>;
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
