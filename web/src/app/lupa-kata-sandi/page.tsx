import type { Metadata } from "next";
import { AuthShell } from "@/components/shell/auth-shell";
import { ForgotForm } from "./forgot-form";

export const metadata: Metadata = { title: "Lupa kata sandi" };

export default function ForgotPasswordPage() {
  return (
    <AuthShell
      title="Lupa kata sandi"
      description="Tulis NIPY, email, atau nomor WA Anda. Tautan untuk membuat kata sandi baru dikirim ke email yang terdaftar di akun."
    >
      <ForgotForm />
    </AuthShell>
  );
}
