import type { Metadata } from "next";
import { ReviewQueue } from "@/components/attendance/review-queue";

export const metadata: Metadata = { title: "Review Absen" };

export default function ReviewAbsenPage() {
  return <ReviewQueue />;
}
