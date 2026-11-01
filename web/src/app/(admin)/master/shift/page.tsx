import type { Metadata } from "next";
import { ComingSoon } from "@/components/ui/layout";

export const metadata: Metadata = { title: "Shift" };

export default function ShiftPage() {
  return (
    <ComingSoon
      title="Shift"
      phase="Fase 2 (Presensi, Desember 2026)"
      items={["Jam masuk & pulang per shift","Toleransi keterlambatan","Penugasan shift per Pejuang","Shift rotasi (backlog)"]}
    />
  );
}
