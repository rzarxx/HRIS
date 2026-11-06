import type { Metadata } from "next";
import { AttendanceAttempts } from "@/components/attendance/attendance-attempts";

export const metadata: Metadata = { title: "Upaya Absen" };

export default function UpayaAbsenPage() {
  return <AttendanceAttempts />;
}
