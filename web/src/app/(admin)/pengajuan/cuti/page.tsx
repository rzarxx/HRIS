import type { Metadata } from "next";
import { LeaveList } from "@/components/requests/leave-list";

export const metadata: Metadata = { title: "Cuti & Izin" };

export default function CutiIzinPage() {
  return <LeaveList />;
}
