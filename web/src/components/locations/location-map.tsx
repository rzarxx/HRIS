"use client";

import L from "leaflet";
import "leaflet/dist/leaflet.css";
import { useEffect, useRef } from "react";

export interface LatLng {
  latitude: number;
  longitude: number;
}

// Warna garis area, sama dengan primary-600.
const AREA_COLOR = "#15803d";
const DEFAULT_VIEW: L.LatLngTuple = [-6.2, 106.8166];
const DEFAULT_ZOOM = 11;
const PIN_ZOOM = 16;

const toLatLng = (p: LatLng): L.LatLngTuple => [p.latitude, p.longitude];

/**
 * Peta pengatur area absen. Komponen ini hanya menggambar dan melaporkan titik yang dipilih;
 * nilainya tetap disimpan di form agar sama dengan isian koordinat.
 * Leaflet butuh `window`, jadi muat lewat next/dynamic dengan ssr: false.
 */
export default function LocationMap({
  mode,
  center,
  radiusM,
  polygon,
  onPick,
  onCenterDrag,
}: {
  mode: "radius" | "polygon";
  center: LatLng | null;
  radiusM: number;
  polygon: LatLng[];
  /** Klik pada peta: titik pusat di mode radius, titik sudut di mode poligon. */
  onPick: (p: LatLng) => void;
  onCenterDrag: (p: LatLng) => void;
}) {
  const box = useRef<HTMLDivElement>(null);
  const map = useRef<L.Map | null>(null);
  const layers = useRef<L.LayerGroup | null>(null);
  const handlers = useRef({ onPick, onCenterDrag });

  useEffect(() => {
    handlers.current = { onPick, onCenterDrag };
  }, [onPick, onCenterDrag]);

  const initial = useRef({ center, radiusM, polygon });

  useEffect(() => {
    if (!box.current) return;
    // Inersia dijalankan di frame berikutnya, padahal peta bisa sudah dihapus karena modal ditutup.
    // Klik ganda juga mematikan zoom karena klik di peta berarti memilih titik.
    const m = L.map(box.current, { inertia: false, doubleClickZoom: false });
    L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      // Nginx memasang no-referrer untuk seluruh web, padahal OpenStreetMap memblokir tile tanpa Referer.
      // Hanya domain asal yang dikirim, bukan alamat halaman.
      referrerPolicy: "strict-origin",
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    }).addTo(m);
    layers.current = L.layerGroup().addTo(m);
    m.on("click", (e: L.LeafletMouseEvent) => handlers.current.onPick({ latitude: e.latlng.lat, longitude: e.latlng.lng }));

    const start = initial.current;
    if (start.polygon.length >= 3) {
      m.fitBounds(L.latLngBounds(start.polygon.map(toLatLng)), { padding: [24, 24] });
    } else if (start.center) {
      m.fitBounds(L.latLng(toLatLng(start.center)).toBounds(start.radiusM * 2), { padding: [24, 24] });
    } else {
      m.setView(DEFAULT_VIEW, DEFAULT_ZOOM);
    }
    map.current = m;

    // Peta berada di dalam modal, jadi ukurannya baru final setelah modal tampil.
    const observer = new ResizeObserver(() => m.invalidateSize());
    observer.observe(box.current);
    return () => {
      observer.disconnect();
      m.remove();
      map.current = null;
      layers.current = null;
    };
  }, []);

  useEffect(() => {
    const m = map.current;
    const group = layers.current;
    if (!m || !group) return;
    group.clearLayers();

    if (mode === "polygon") {
      const points = polygon.map(toLatLng);
      if (points.length >= 3) L.polygon(points, { color: AREA_COLOR, weight: 2, fillOpacity: 0.15 }).addTo(group);
      else if (points.length === 2) L.polyline(points, { color: AREA_COLOR, weight: 2 }).addTo(group);
      for (const p of points) L.circleMarker(p, { radius: 4, color: AREA_COLOR, fillColor: "#ffffff", fillOpacity: 1, weight: 2 }).addTo(group);
      return;
    }

    if (!center) return;
    const position = toLatLng(center);
    L.circle(position, { radius: radiusM, color: AREA_COLOR, weight: 2, fillOpacity: 0.15 }).addTo(group);
    const marker = L.marker(position, {
      draggable: true,
      title: "Titik pusat lokasi",
      alt: "Titik pusat lokasi",
      icon: L.divIcon({
        className: "",
        html: '<span class="block size-4 rounded-full border-2 border-white bg-primary-600 shadow"></span>',
        iconSize: [16, 16],
        iconAnchor: [8, 8],
      }),
    }).addTo(group);
    marker.on("dragend", () => {
      const { lat, lng } = marker.getLatLng();
      handlers.current.onCenterDrag({ latitude: lat, longitude: lng });
    });
    // Titik dari pencarian atau ketikan bisa berada di luar tampilan.
    if (!m.getBounds().contains(position)) m.setView(position, PIN_ZOOM);
  }, [mode, center, radiusM, polygon]);

  return <div ref={box} role="application" aria-label="Peta lokasi kantor" className="h-72 w-full rounded-lg border border-slate-200" />;
}
