import type { Metadata } from "next";
import { ComingSoon } from "@/components/ui/layout";

export const metadata: Metadata = { title: "Surat Peringatan" };

export default function SuratPeringatanPage() {
  return (
    <ComingSoon
      title="Surat Peringatan"
      items={["Terbitkan SP1, SP2, SP3","Lampiran dokumen SP","Masa berlaku SP","Tampil di akun Pejuang"]}
    />
  );
}
