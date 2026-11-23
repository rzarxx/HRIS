import type { Metadata } from "next";
import { AuthShell } from "@/components/shell/auth-shell";
import { ConfirmEmailForm } from "./confirm-form";

// Token ada di fragmen URL (#token=…) sehingga tidak terkirim ke server mana pun; no-referrer menjaga
// alamat halaman ini tidak ikut ke tautan keluar (D59).
export const metadata: Metadata = { title: "Konfirmasi email", referrer: "no-referrer" };

export default function ConfirmEmailPage() {
  return (
    <AuthShell title="Konfirmasi email baru" description="Email baru dipakai untuk masuk dan menerima tautan akun setelah dikonfirmasi.">
      <ConfirmEmailForm />
    </AuthShell>
  );
}
