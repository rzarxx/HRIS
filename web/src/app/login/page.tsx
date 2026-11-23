import type { Metadata } from "next";
import { Suspense } from "react";
import { AuthShell } from "@/components/shell/auth-shell";
import { LoginForm } from "./login-form";

export const metadata: Metadata = { title: "Masuk" };

export default function LoginPage() {
  return (
    <AuthShell title="Masuk ke akun Anda" description="Masuk dengan NIPY, email, atau nomor WA yang terdaftar di SDM.">
      <Suspense>
        <LoginForm />
      </Suspense>
    </AuthShell>
  );
}
