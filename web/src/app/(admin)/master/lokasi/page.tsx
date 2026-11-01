import type { Metadata } from "next";
import { ComingSoon } from "@/components/ui/layout";

export const metadata: Metadata = { title: "Lokasi Kantor" };

export default function LokasiKantorPage() {
  return (
    <ComingSoon
      title="Lokasi Kantor"
      phase="Fase 2 (Presensi, Desember 2026)"
      items={["Titik & radius geofence per kantor","Poligon area kantor (lebih presisi)","Peta interaktif untuk menentukan lokasi","Penugasan lokasi per Pejuang"]}
    />
  );
}
