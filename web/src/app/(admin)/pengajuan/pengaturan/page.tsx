import type { Metadata } from "next";
import { RequestSettings } from "@/components/requests/request-settings";

export const metadata: Metadata = { title: "Jenis & Alur Pengajuan" };

export default function PengaturanPengajuanPage() {
  return <RequestSettings />;
}
