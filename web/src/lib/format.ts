// Waktu disimpan UTC, ditampilkan WIB (DESIGN.md §10).
export const TZ = "Asia/Jakarta";

const rupiahFmt = new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 });
const numberFmt = new Intl.NumberFormat("id-ID");

/** Uang dari API berupa string; konversi ke number hanya untuk tampilan. */
export const rupiah = (n: number | string) => rupiahFmt.format(Number(n));

/** Menjumlah string uang tanpa floating point, hasilnya string 2 digit. */
export function sumMoney(values: string[]) {
  const cents = values.reduce((total, v) => {
    const [whole, frac = ""] = v.split(".");
    return total + BigInt(whole) * 100n + BigInt(frac.padEnd(2, "0").slice(0, 2));
  }, 0n);
  return `${cents / 100n}.${String(cents % 100n).padStart(2, "0")}`;
}
export const number = (n: number) => numberFmt.format(n);
export const percent = (r: number) => `${(r * 100).toLocaleString("id-ID", { maximumFractionDigits: 1 })}%`;

// Urutan mengikuti nilai hari di API: 1=Senin sampai 7=Ahad.
export const DAYS = ["Sen", "Sel", "Rab", "Kam", "Jum", "Sab", "Ahd"];

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

/** Rentang tanggal ringkas: "3–5 Januari 2027", "30 Januari – 2 Februari 2027". */
export function dateRange(start: string, end: string) {
  if (start === end) return dateLong(start);
  const [ys, ms, ds] = start.split("-").map(Number);
  const [ye, me, de] = end.split("-").map(Number);
  if (ys === ye && ms === me) return `${ds}–${de} ${MONTHS[me - 1]} ${ye}`;
  if (ys === ye) return `${ds} ${MONTHS[ms - 1]} – ${de} ${MONTHS[me - 1]} ${ye}`;
  return `${dateLong(start)} – ${dateLong(end)}`;
}

/** Waktu relatif singkat untuk kotak masuk: "baru saja", "5 menit lalu", "2 hari lalu". */
export function timeAgo(iso: string, now = Date.now()) {
  const minutes = Math.floor((now - new Date(iso).getTime()) / 60_000);
  if (minutes < 1) return "baru saja";
  if (minutes < 60) return `${minutes} menit lalu`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} jam lalu`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days} hari lalu`;
  return dateLong(iso);
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
