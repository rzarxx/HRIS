import type { Metadata } from "next";
import { AttendanceMonitor } from "@/components/attendance/attendance-monitor";

export const metadata: Metadata = { title: "Monitor Absen" };

export default function MonitorAbsenPage() {
  return <AttendanceMonitor />;
}
