import type { Metadata } from "next";
import { ComingSoon } from "@/components/ui/layout";

export const metadata: Metadata = { title: "Review Absen" };

export default function ReviewAbsenPage() {
  return (
    <ComingSoon
      title="Review Absen"
      phase="Fase 2 (Presensi, Desember 2026)"
      items={["Antrian absen dengan skor risiko 30-59","Lihat selfie, lokasi, dan sinyal risiko","Setujui atau tolak absen","Persetujuan ganti perangkat"]}
    />
  );
}
