import type { Metadata } from "next";
import { ReviewQueue } from "@/components/attendance/review-queue";

export const metadata: Metadata = { title: "Review Presensi" };

export default function ReviewPresensiPage() {
  return <ReviewQueue />;
}
