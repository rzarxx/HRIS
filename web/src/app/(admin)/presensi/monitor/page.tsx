import type { Metadata } from "next";
import { AttendanceMonitor } from "@/components/attendance/attendance-monitor";

export const metadata: Metadata = { title: "Monitor Presensi" };

export default function MonitorPresensiPage() {
  return <AttendanceMonitor />;
}
