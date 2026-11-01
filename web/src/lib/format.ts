// Waktu disimpan UTC, ditampilkan WIB (DESIGN.md §10).
export const TZ = "Asia/Jakarta";

const rupiahFmt = new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 });
const numberFmt = new Intl.NumberFormat("id-ID");

export const rupiah = (n: number) => rupiahFmt.format(n);
export const number = (n: number) => numberFmt.format(n);
export const percent = (r: number) => `${(r * 100).toLocaleString("id-ID", { maximumFractionDigits: 1 })}%`;

export function dateLong(iso: string | null | undefined) {
  if (!iso) return "-";
  const d = iso.length === 10 ? new Date(`${iso}T00:00:00+07:00`) : new Date(iso);
  return d.toLocaleDateString("id-ID", { timeZone: TZ, day: "numeric", month: "long", year: "numeric" });
}

export function dateTime(iso: string | null | undefined) {
  if (!iso) return "-";
  return new Date(iso).toLocaleString("id-ID", {
    timeZone: TZ,
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export const MONTHS = [
  "Januari", "Februari", "Maret", "April", "Mei", "Juni",
  "Juli", "Agustus", "September", "Oktober", "November", "Desember",
];

/** Tanggal hari ini (WIB) dalam format YYYY-MM-DD. */
export function todayWib(now = new Date()) {
  return now.toLocaleDateString("en-CA", { timeZone: TZ });
}

/** ISO UTC -> nilai input datetime-local dalam WIB. */
export function toWibInput(iso: string | null) {
  if (!iso) return "";
  const d = new Date(new Date(iso).getTime() + 7 * 3600_000);
  return d.toISOString().slice(0, 16);
}

/** Nilai input datetime-local (WIB) -> ISO UTC. */
export function fromWibInput(v: string) {
  return v ? new Date(`${v}:00+07:00`).toISOString() : null;
}

export function initials(name: string) {
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");
}
