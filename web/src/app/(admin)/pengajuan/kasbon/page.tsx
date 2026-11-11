import type { Metadata } from "next";
import { ComingSoon } from "@/components/ui/layout";

export const metadata: Metadata = { title: "Pinjaman" };

export default function KasbonPage() {
  return (
    <ComingSoon
      title="Pinjaman"
      items={["Pengajuan pinjaman dari aplikasi HP","Approval Subdiv lalu Kadiv","Jadwal cicilan","Potongan otomatis di payroll"]}
    />
  );
}
