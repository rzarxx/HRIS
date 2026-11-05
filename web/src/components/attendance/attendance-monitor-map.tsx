"use client";

import L from "leaflet";
import "leaflet/dist/leaflet.css";
import { useEffect, useRef } from "react";
import { zoomWheelAfterClick } from "@/lib/map";

export interface MonitorPoint {
  id: string;
  name: string;
  detail: string;
  color: string;
  latitude: number;
  longitude: number;
}

export interface OfficeArea {
  id: string;
  name: string;
  latitude: number;
  longitude: number;
  radius_m: number;
}

const OFFICE_COLOR = "#475569";
const DEFAULT_VIEW: L.LatLngTuple = [-6.2, 106.8166];
const DEFAULT_ZOOM = 11;
const MAX_FIT_ZOOM = 16;

// Isi popup memakai textContent karena nama berasal dari data pengguna, sedangkan Leaflet menulis string sebagai HTML.
function popupOf(name: string, detail: string) {
  const box = document.createElement("div");
  const title = document.createElement("p");
  title.style.fontWeight = "600";
  title.style.margin = "0";
  title.textContent = name;
  const line = document.createElement("p");
  line.style.margin = "2px 0 0";
  line.textContent = detail;
  box.append(title, line);
  return box;
}

/**
 * Peta titik absen. Tampilan hanya disesuaikan saat isi saringan berubah (fitKey), bukan setiap
 * pembaruan data, agar peta yang sedang digeser tidak melompat.
 * Leaflet butuh `window`, jadi muat lewat next/dynamic dengan ssr: false.
 */
export default function AttendanceMonitorMap({ points, offices, fitKey }: { points: MonitorPoint[]; offices: OfficeArea[]; fitKey: string }) {
  const box = useRef<HTMLDivElement>(null);
  const map = useRef<L.Map | null>(null);
  const layer = useRef<L.LayerGroup | null>(null);
  const fitted = useRef("");

  useEffect(() => {
    if (!box.current) return;
    // Di ponsel satu jari menggulir halaman, dua jari menggeser peta.
    const m = L.map(box.current, { inertia: false, scrollWheelZoom: false, dragging: !L.Browser.mobile }).setView(DEFAULT_VIEW, DEFAULT_ZOOM);
    zoomWheelAfterClick(m);
    L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      // Nginx memasang no-referrer untuk seluruh web, padahal OpenStreetMap memblokir tile tanpa Referer.
      referrerPolicy: "strict-origin",
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    }).addTo(m);
    layer.current = L.layerGroup().addTo(m);
    map.current = m;
    const observer = new ResizeObserver(() => m.invalidateSize({ animate: false }));
    observer.observe(box.current);
    return () => {
      observer.disconnect();
      m.remove();
      map.current = null;
      layer.current = null;
    };
  }, []);

  useEffect(() => {
    const m = map.current;
    const group = layer.current;
    if (!m || !group) return;
    group.clearLayers();
    for (const o of offices) {
      L.circle([o.latitude, o.longitude], { radius: o.radius_m, color: OFFICE_COLOR, weight: 1, dashArray: "4 4", fillOpacity: 0.05, interactive: false }).addTo(group);
    }
    for (const p of points) {
      L.circleMarker([p.latitude, p.longitude], { radius: 7, color: "#ffffff", weight: 2, fillColor: p.color, fillOpacity: 1 })
        .bindPopup(popupOf(p.name, p.detail))
        .addTo(group);
    }
    if (fitted.current !== fitKey && points.length > 0) {
      fitted.current = fitKey;
      m.fitBounds(L.latLngBounds(points.map((p): L.LatLngTuple => [p.latitude, p.longitude])), { padding: [40, 40], maxZoom: MAX_FIT_ZOOM, animate: false });
    }
  }, [points, offices, fitKey]);

  return <div ref={box} role="img" aria-label={`Peta titik absen, ${points.length} Pejuang`} className="h-80 w-full rounded-lg border border-slate-200" />;
}
