import type { Metadata } from "next";
import { ComingSoon } from "@/components/ui/layout";

export const metadata: Metadata = { title: "Cuti & Izin" };

export default function CutiIzinPage() {
  return (
    <ComingSoon
      title="Cuti & Izin"
      phase="Fase 4 (Pengajuan & Approval, Januari 2027)"
      items={["Form cuti/izin dari aplikasi HP + lampiran","Approval Subdiv lalu SDM","Saldo cuti tahunan","Status otomatis di rekap presensi"]}
    />
  );
}
