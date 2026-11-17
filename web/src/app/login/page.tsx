import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { Logo } from "@/components/logo";
import { SiteFooter } from "@/components/shell/site-footer";
import { LoginForm } from "./login-form";

export const metadata: Metadata = { title: "Masuk" };

export default function LoginPage() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-slate-50 px-4">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex justify-center">
          <Logo size="lg" />
        </div>
        <div className="rounded-lg border border-t-4 border-slate-200 border-t-secondary-400 bg-white p-6 shadow-sm">
          <h1 className="text-lg font-semibold text-slate-900">Masuk ke akun Anda</h1>
          <p className="mt-1 mb-5 text-sm text-slate-500">Masuk dengan NIPY, email, atau nomor WA yang terdaftar di SDM.</p>
          <Suspense>
            <LoginForm />
          </Suspense>
        </div>
        <p className="mt-4 text-center text-xs">
          <Link href="/kebijakan-privasi/" className="text-slate-500 hover:text-slate-700 hover:underline">
            Kebijakan Privasi
          </Link>
        </p>
        <SiteFooter className="mt-0 pt-2" />
      </div>
    </main>
  );
}
