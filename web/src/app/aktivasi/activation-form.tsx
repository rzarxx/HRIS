"use client";

import { CheckCircle2 } from "lucide-react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/form";
import { api, ApiError } from "@/lib/api-client";

export function ActivationForm() {
  const token = useSearchParams().get("token") ?? "";
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState(token ? "" : "Tautan aktivasi tidak lengkap. Minta tautan baru ke SDM.");
  const [loading, setLoading] = useState(false);
  const [done, setDone] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    const next: Record<string, string> = {};
    if (password.length < 10) next.password = "Kata sandi minimal 10 karakter.";
    if (password !== confirm) next.confirm = "Konfirmasi tidak sama.";
    setErrors(next);
    if (Object.keys(next).length) return;

    setLoading(true);
    try {
      await api.post("/auth/activate", { token, password });
      setDone(true);
    } catch (err) {
      if (err instanceof ApiError && err.fields) setErrors(err.fields);
      setFormError(err instanceof ApiError ? err.message : "Tidak dapat terhubung ke server.");
    } finally {
      setLoading(false);
    }
  }

  if (done) {
    return (
      <div className="space-y-4 text-center">
        <CheckCircle2 className="mx-auto size-10 text-emerald-600" aria-hidden />
        <p className="text-sm text-slate-700">Akun Anda sudah aktif. Silakan masuk dengan kata sandi baru.</p>
        <Link href="/login" className="inline-block text-sm font-medium text-primary-600 hover:underline">
          Ke halaman masuk
        </Link>
      </div>
    );
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-4">
      {formError && (
        <div role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {formError}
        </div>
      )}
      <Field label="Kata sandi baru" htmlFor="password" error={errors.password} hint="Minimal 10 karakter, hindari kata sandi umum.">
        <Input
          id="password"
          type="password"
          autoComplete="new-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          invalid={!!errors.password}
          disabled={!token}
        />
      </Field>
      <Field label="Ulangi kata sandi" htmlFor="confirm" error={errors.confirm}>
        <Input
          id="confirm"
          type="password"
          autoComplete="new-password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          invalid={!!errors.confirm}
          disabled={!token}
        />
      </Field>
      <Button type="submit" loading={loading} disabled={!token} className="w-full">
        Aktifkan akun
      </Button>
    </form>
  );
}
