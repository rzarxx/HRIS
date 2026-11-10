import type { Metadata } from "next";
import { ApprovalInbox } from "@/components/requests/approval-inbox";

export const metadata: Metadata = { title: "Persetujuan" };

export default function PersetujuanPage() {
  return <ApprovalInbox />;
}
