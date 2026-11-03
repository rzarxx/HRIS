"use client";

import { Search } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/form";
import type { LatLng } from "./location-map";

interface Place extends LatLng {
  name: string;
}

// Nominatim milik OpenStreetMap. Hanya dipanggil saat tombol ditekan, bukan saat mengetik,
// karena kebijakan layanannya melarang pencarian otomatis.
async function searchPlaces(query: string): Promise<Place[]> {
  const url = new URL("https://nominatim.openstreetmap.org/search");
  url.search = new URLSearchParams({ format: "jsonv2", limit: "5", countrycodes: "id", q: query }).toString();
  // Sama seperti tile peta: kirim domain asal saja karena halaman memakai no-referrer.
  const res = await fetch(url, { credentials: "omit", referrerPolicy: "strict-origin" });
  if (!res.ok) throw new Error(`status ${res.status}`);
  const rows = (await res.json()) as { lat: string; lon: string; display_name: string }[];
  return rows
    .map((r) => ({ latitude: Number(r.lat), longitude: Number(r.lon), name: String(r.display_name) }))
    .filter((p) => Number.isFinite(p.latitude) && Number.isFinite(p.longitude));
}

export function PlaceSearch({ onSelect }: { onSelect: (p: LatLng) => void }) {
  const [query, setQuery] = useState("");
  const [places, setPlaces] = useState<Place[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);

  async function run() {
    const q = query.trim();
    if (!q || loading) return;
    setLoading(true);
    setError(false);
    try {
      setPlaces(await searchPlaces(q));
    } catch {
      setPlaces(null);
      setError(true);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div>
      <div className="flex gap-2">
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            // Kotak ini berada di dalam form lokasi; Enter tidak boleh menyimpan form.
            if (e.key === "Enter") {
              e.preventDefault();
              void run();
            }
          }}
          aria-label="Cari alamat atau nama tempat"
          placeholder="Cari alamat atau nama tempat..."
        />
        <Button type="button" variant="secondary" onClick={run} loading={loading}>
          <Search className="size-4" /> Cari
        </Button>
      </div>
      {error && <p className="mt-1 text-xs text-red-600">Pencarian gagal. Klik langsung pada peta atau ketik koordinatnya.</p>}
      {places?.length === 0 && <p className="mt-1 text-xs text-slate-500">Tempat tidak ditemukan. Coba kata kunci lain.</p>}
      {places && places.length > 0 && (
        <ul className="mt-2 divide-y divide-slate-100 rounded-lg border border-slate-200">
          {places.map((p) => (
            <li key={`${p.latitude},${p.longitude},${p.name}`}>
              <button
                type="button"
                className="w-full px-3 py-2 text-left text-sm text-slate-700 hover:bg-slate-50"
                onClick={() => {
                  onSelect(p);
                  setPlaces(null);
                }}
              >
                {p.name}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
