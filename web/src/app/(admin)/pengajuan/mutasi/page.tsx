import type { Metadata } from "next";
import { ComingSoon } from "@/components/ui/layout";

export const metadata: Metadata = { title: "Mutasi" };

export default function MutasiPage() {
  return (
    <ComingSoon
      title="Mutasi"
      phase="Fase 4 (Pengajuan & Approval, Januari 2027)"
      items={["Pengajuan mutasi antar subdiv","Approval Subdiv asal, Kadiv, dan SDM","Tanggal efektif mutasi","Perubahan subdiv otomatis"]}
    />
  );
}
