import type L from "leaflet";

/**
 * Roda mouse baru memperbesar peta setelah peta diklik, supaya halaman tetap bisa digulir saat
 * kursor melewati peta. Pakai bersama opsi scrollWheelZoom: false.
 */
export function zoomWheelAfterClick(m: L.Map) {
  m.on("focus", () => m.scrollWheelZoom.enable());
  m.on("blur", () => m.scrollWheelZoom.disable());
}
