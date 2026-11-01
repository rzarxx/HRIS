// Skema validasi form, dipakai di form (client) dan API sementara (server).
import { z } from "zod";

const required = (label: string) => z.string({ required_error: `${label} wajib diisi.` }).trim().min(1, `${label} wajib diisi.`);
const optionalText = z
  .string()
  .trim()
  .transform((v) => (v === "" ? null : v))
  .nullable()
  .optional()
  .transform((v) => v ?? null);

export const loginSchema = z.object({
  email: required("Email").email("Format email tidak valid."),
  password: required("Password"),
});

export const divisionSchema = z.object({
  code: required("Kode")
    .max(10, "Kode maksimal 10 karakter.")
    .regex(/^[A-Za-z0-9-]+$/, "Kode hanya huruf, angka, dan tanda minus.")
    .transform((v) => v.toUpperCase()),
  name: required("Nama divisi").max(80, "Nama maksimal 80 karakter."),
  head_employee_id: z.string().uuid().nullable().optional().transform((v) => v ?? null),
});

export const subdivisionSchema = z.object({
  division_id: required("Divisi").uuid("Pilih divisi."),
  code: required("Kode")
    .max(12, "Kode maksimal 12 karakter.")
    .regex(/^[A-Za-z0-9-]+$/, "Kode hanya huruf, angka, dan tanda minus.")
    .transform((v) => v.toUpperCase()),
  name: required("Nama subdiv").max(80, "Nama maksimal 80 karakter."),
  head_employee_id: z.string().uuid().nullable().optional().transform((v) => v ?? null),
});

export const employeeSchema = z.object({
  nip: optionalText.refine((v) => v === null || /^[A-Za-z0-9-]{4,20}$/.test(v), "NIP 4-20 karakter huruf/angka."),
  full_name: required("Nama lengkap").min(3, "Nama minimal 3 karakter.").max(100, "Nama maksimal 100 karakter."),
  email: optionalText.refine((v) => v === null || z.string().email().safeParse(v).success, "Format email tidak valid."),
  phone: optionalText.refine((v) => v === null || /^(\+62|0)8[0-9]{7,12}$/.test(v), "Nomor HP tidak valid (contoh 081234567890)."),
  subdivision_id: required("Subdiv").uuid("Pilih subdiv."),
  position: optionalText,
  join_date: optionalText.refine((v) => v === null || /^\d{4}-\d{2}-\d{2}$/.test(v), "Tanggal tidak valid."),
  base_salary: z.coerce
    .number({ invalid_type_error: "Gaji pokok harus angka." })
    .min(0, "Gaji pokok tidak boleh negatif.")
    .max(1_000_000_000, "Gaji pokok terlalu besar."),
  status: z.enum(["active", "inactive"]).default("active"),
});

export const announcementSchema = z
  .object({
    title: required("Judul").max(120, "Judul maksimal 120 karakter."),
    body: required("Isi").max(2000, "Isi maksimal 2000 karakter."),
    type: z.enum(["popup", "banner"], { required_error: "Pilih tipe." }),
    division_id: z.string().uuid().nullable().optional().transform((v) => v ?? null),
    starts_at: required("Mulai tayang").datetime({ message: "Tanggal mulai tidak valid." }),
    ends_at: z.string().datetime({ message: "Tanggal berakhir tidak valid." }).nullable().optional().transform((v) => v ?? null),
    is_active: z.boolean().default(true),
  })
  .refine((v) => !v.ends_at || v.ends_at > v.starts_at, { path: ["ends_at"], message: "Berakhir harus setelah mulai tayang." });

export type DivisionInput = z.input<typeof divisionSchema>;
export type SubdivisionInput = z.input<typeof subdivisionSchema>;
export type EmployeeInput = z.input<typeof employeeSchema>;
export type AnnouncementInput = z.input<typeof announcementSchema>;

/** Validasi di client; mengembalikan pesan error per field atau null bila valid. */
export function validate<T>(schema: z.ZodType<T, z.ZodTypeDef, unknown>, values: unknown): Record<string, string> | null {
  const r = schema.safeParse(values);
  if (r.success) return null;
  const fields: Record<string, string> = {};
  for (const issue of r.error.issues) fields[issue.path.join(".")] ??= issue.message;
  return fields;
}
