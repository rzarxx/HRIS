import type { Metadata } from "next";
import { Suspense } from "react";
import { Logo } from "@/components/logo";
import { ActivationForm } from "./activation-form";

export const metadata: Metadata = { title: "Aktivasi akun", robots: { index: false } };

export default function ActivationPage() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-slate-50 px-4">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex justify-center">
          <Logo />
        </div>
        <div className="rounded-lg border border-t-4 border-slate-200 border-t-secondary-400 bg-white p-6 shadow-sm">
          <h1 className="text-lg font-semibold text-slate-900">Aktivasi akun</h1>
          <p className="mt-1 mb-5 text-sm text-slate-500">Tetapkan kata sandi Anda. Hanya Anda yang mengetahuinya.</p>
          <Suspense>
            <ActivationForm />
          </Suspense>
        </div>
      </div>
    </main>
  );
}
