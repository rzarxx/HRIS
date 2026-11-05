import type { Metadata } from "next";
import { Suspense } from "react";
import { Logo } from "@/components/logo";
import { LoginForm } from "./login-form";

export const metadata: Metadata = { title: "Masuk" };

export default function LoginPage() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-slate-50 px-4">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex justify-center">
          <Logo />
        </div>
        <div className="rounded-lg border border-t-4 border-slate-200 border-t-secondary-400 bg-white p-6 shadow-sm">
          <h1 className="text-lg font-semibold text-slate-900">Masuk ke akun Anda</h1>
          <p className="mt-1 mb-5 text-sm text-slate-500">Gunakan email dan password yang terdaftar di SDM.</p>
          <Suspense>
            <LoginForm />
          </Suspense>
        </div>
        <p className="mt-6 text-center text-xs text-slate-400">© 2026 HRIS Pejuang</p>
      </div>
    </main>
  );
}
