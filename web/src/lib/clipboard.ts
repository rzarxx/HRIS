/**
 * Menyalin teks ke clipboard dan menjawab apakah berhasil.
 * navigator.clipboard hanya ada di HTTPS, sedangkan server development masih HTTP (kartu Bug & Error),
 * jadi execCommand tetap dipakai sebagai cadangan.
 */
export async function copyText(text: string) {
  if (navigator.clipboard) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Izin ditolak browser, lanjut ke cara cadangan.
    }
  }
  return copyWithSelection(text);
}

function copyWithSelection(text: string) {
  const area = document.createElement("textarea");
  area.value = text;
  area.readOnly = true;
  area.setAttribute("aria-hidden", "true");
  area.style.position = "fixed";
  area.style.opacity = "0";
  // Di luar dialog yang terbuka, elemen tidak bisa difokus, jadi harus ditempel di dalam dialog itu.
  const host = document.activeElement?.closest("dialog") ?? document.body;
  const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  host.append(area);
  area.select();
  try {
    return document.execCommand("copy");
  } catch {
    return false;
  } finally {
    area.remove();
    previous?.focus();
  }
}
