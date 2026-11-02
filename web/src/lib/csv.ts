// Ekspor CSV yang langsung terbuka rapi di Excel berlokal Indonesia:
// pemisah titik koma + BOM UTF-8.

/**
 * Menetralkan formula injection (MASTER.md §5.8): sel teks yang diawali karakter formula
 * diberi awalan tanda kutip agar Excel membacanya sebagai teks, bukan rumus.
 */
function neutralize(value: string | number) {
  if (typeof value === "number") return String(value);
  return /^[=+\-@\t\r]/.test(value) ? `'${value}` : value;
}

export function toCsv(header: string[], rows: (string | number)[][]) {
  const cell = (v: string | number) => {
    const s = neutralize(v);
    return /[";\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
  };
  return [header, ...rows].map((r) => r.map(cell).join(";")).join("\r\n");
}

export function downloadCsv(filename: string, header: string[], rows: (string | number)[][]) {
  const blob = new Blob(["﻿" + toCsv(header, rows)], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
