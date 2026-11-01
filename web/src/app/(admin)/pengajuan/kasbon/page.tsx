import type { Metadata } from "next";
import { ComingSoon } from "@/components/ui/layout";

export const metadata: Metadata = { title: "Kasbon" };

export default function KasbonPage() {
  return (
    <ComingSoon
      title="Kasbon"
      phase="Fase 4 (Pengajuan & Approval, Januari 2027)"
      items={["Pengajuan kasbon dari aplikasi HP","Approval Subdiv lalu Kadiv","Jadwal cicilan","Potongan otomatis di payroll"]}
    />
  );
}
