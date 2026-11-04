"use client";

import L from "leaflet";
import "leaflet/dist/leaflet.css";
import { useEffect, useRef } from "react";

export interface MapPoint {
  label: string;
  latitude: number;
  longitude: number;
}

// Warna titik, sama dengan primary-600 dan slate-600.
const COLORS = ["#15803d", "#475569"];
const POINT_ZOOM = 16;

/**
 * Peta baca saja untuk titik absen. Leaflet butuh `window`, jadi muat lewat next/dynamic dengan ssr: false.
 */
export default function AttendancePointsMap({ points }: { points: MapPoint[] }) {
  const box = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!box.current || points.length === 0) return;
    const m = L.map(box.current, { inertia: false });
    L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      // Nginx memasang no-referrer untuk seluruh web, padahal OpenStreetMap memblokir tile tanpa Referer.
      referrerPolicy: "strict-origin",
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    }).addTo(m);
    const latLngs = points.map((p): L.LatLngTuple => [p.latitude, p.longitude]);
    points.forEach((p, i) => {
      L.circleMarker(latLngs[i], { radius: 7, color: "#ffffff", weight: 2, fillColor: COLORS[i % COLORS.length], fillOpacity: 1 })
        .bindTooltip(p.label, { permanent: true, direction: "top", offset: [0, -6] })
        .addTo(m);
    });
    const fit = () => {
      m.invalidateSize({ animate: false });
      m.fitBounds(L.latLngBounds(latLngs), { padding: [40, 40], maxZoom: POINT_ZOOM, animate: false });
    };
    fit();
    // Peta bisa dibuat sebelum dialog tampil, saat ukurannya masih nol.
    const observer = new ResizeObserver(fit);
    observer.observe(box.current);
    return () => {
      observer.disconnect();
      m.remove();
    };
  }, [points]);

  return <div ref={box} role="img" aria-label={`Peta titik absen: ${points.map((p) => p.label).join(", ")}`} className="h-56 w-full rounded-lg border border-slate-200" />;
}
