import type { Metadata } from "next";
import { AttendanceAttempts } from "@/components/attendance/attendance-attempts";

export const metadata: Metadata = { title: "Upaya Presensi" };

export default function UpayaPresensiPage() {
  return <AttendanceAttempts />;
}
