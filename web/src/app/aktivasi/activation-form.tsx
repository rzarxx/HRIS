"use client";

import { CheckCircle2, CircleAlert, Link2Off, TimerOff } from "lucide-react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/form";
import { Loading } from "@/components/ui/layout";
import { api, ApiError } from "@/lib/api-client";

const BLOCKED = {
  ACTIVATION_USED: { title: "Tautan sudah tidak berlaku", icon: Link2Off },
  ACTIVATION_EXPIRED: { title: "Tautan sudah kedaluwarsa", icon: TimerOff },
  ACTIVATION_INVALID: { title: "Tautan tidak valid", icon: CircleAlert },
} as const;
type BlockedCode = keyof typeof BLOCKED;

const blockedCode = (err: unknown) => (err instanceof ApiError && err.code in BLOCKED ? (err.code as BlockedCode) : null);

function Result({ icon: Icon, tone, title, message }: { icon: typeof CircleAlert; tone: string; title: string; message: string }) {
  return (
    <div className="space-y-3 text-center">
      <Icon className={`mx-auto size-10 ${tone}`} aria-hidden />
      <h1 className="text-lg font-semibold text-slate-900">{title}</h1>
      <p role="status" className="text-sm text-slate-600">
        {message}
      </p>
      <Link href="/login" className="inline-block text-sm font-medium text-primary-700 hover:underline">
        Ke halaman masuk
      </Link>
    </div>
  );
}

export function ActivationForm() {
  const token = useSearchParams().get("token") ?? "";
  const [checking, setChecking] = useState(Boolean(token));
  const [blocked, setBlocked] = useState<{ code: BlockedCode; message: string } | null>(
    token ? null : { code: "ACTIVATION_INVALID", message: "Tautan aktivasi tidak lengkap. Minta tautan baru ke SDM." },
  );
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState("");
  const [loading, setLoading] = useState(false);
  const [done, setDone] = useState(false);

  useEffect(() => {
    if (!token) return;
    let active = true;
    api
      .post("/auth/activate/check", { token })
      .catch((err) => {
        if (!active) return;
        const code = blockedCode(err);
        // Gangguan jaringan atau batas permintaan tidak berarti tautannya salah, dan API tetap memeriksa saat dikirim.
        if (code) setBlocked({ code, message: err.message });
      })
      .finally(() => {
        if (active) setChecking(false);
      });
    return () => {
      active = false;
    };
  }, [token]);

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
      const code = blockedCode(err);
      if (code) {
        setBlocked({ code, message: (err as ApiError).message });
        return;
      }
      if (err instanceof ApiError && err.fields) setErrors(err.fields);
      setFormError(err instanceof ApiError ? err.message : "Tidak dapat terhubung ke server.");
    } finally {
      setLoading(false);
    }
  }

  if (done) {
    return <Result icon={CheckCircle2} tone="text-emerald-600" title="Akun Anda sudah aktif" message="Silakan masuk dengan kata sandi baru. Tautan ini tidak bisa dipakai lagi." />;
  }
  if (blocked) {
    return <Result icon={BLOCKED[blocked.code].icon} tone="text-amber-600" title={BLOCKED[blocked.code].title} message={blocked.message} />;
  }
  if (checking) {
    return <Loading label="Memeriksa tautan..." />;
  }

  return (
    <>
      <h1 className="text-lg font-semibold text-slate-900">Aktivasi akun</h1>
      <p className="mt-1 mb-5 text-sm text-slate-500">Tetapkan kata sandi Anda. Hanya Anda yang mengetahuinya.</p>
      <form onSubmit={submit} noValidate className="space-y-4">
        {formError && (
          <div role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {formError}
          </div>
        )}
        <Field label="Kata sandi baru" htmlFor="password" error={errors.password} hint="Minimal 10 karakter, hindari kata sandi umum.">
          <Input id="password" type="password" autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} invalid={!!errors.password} />
        </Field>
        <Field label="Ulangi kata sandi" htmlFor="confirm" error={errors.confirm}>
          <Input id="confirm" type="password" autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} invalid={!!errors.confirm} />
        </Field>
        <Button type="submit" loading={loading} className="w-full">
          Aktifkan akun
        </Button>
      </form>
    </>
  );
}
