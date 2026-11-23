import type { Metadata } from "next";
import { AuthShell } from "@/components/shell/auth-shell";
import { ResetForm } from "./reset-form";

// Token ada di fragmen URL (#token=…) sehingga tidak terkirim ke server mana pun; no-referrer menjaga
// alamat halaman ini tidak ikut ke tautan keluar.
export const metadata: Metadata = { title: "Atur ulang kata sandi", referrer: "no-referrer" };

export default function ResetPasswordPage() {
  return (
    <AuthShell title="Buat kata sandi baru" description="Kata sandi baru berlaku di web dan aplikasi. Semua sesi lama akan keluar.">
      <ResetForm />
    </AuthShell>
  );
}
