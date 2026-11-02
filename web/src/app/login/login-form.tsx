"use client";

import { Eye, EyeOff, ShieldCheck } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/form";
import { api, ApiError } from "@/lib/api-client";
import { loginSchema, validate } from "@/lib/schemas";

const DEMO = process.env.NEXT_PUBLIC_DEMO_MODE === "1";

export function LoginForm() {
  const next = useSearchParams().get("next");
  const [email, setEmail] = useState(DEMO ? "sdm@pejuang.id" : "");
  const [password, setPassword] = useState("");
  const [show, setShow] = useState(false);
  const [mfaToken, setMfaToken] = useState("");
  const [code, setCode] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState("");
  const [loading, setLoading] = useState(false);

  function goToApp() {
    // Hanya izinkan redirect ke path internal
    window.location.href = next && next.startsWith("/") && !next.startsWith("//") ? next : "/";
  }

  async function submitPassword(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    const fieldErrors = validate(loginSchema, { email, password });
    setErrors(fieldErrors ?? {});
    if (fieldErrors) return;

    setLoading(true);
    try {
      await api.post("/auth/web/login", { email, password });
      goToApp();
    } catch (err) {
      setLoading(false);
      if (err instanceof ApiError && err.code === "AUTH_MFA_REQUIRED" && typeof err.details?.mfa_token === "string") {
        setMfaToken(err.details.mfa_token);
        setPassword("");
        return;
      }
      if (err instanceof ApiError && err.fields) setErrors(err.fields);
      setFormError(err instanceof ApiError ? err.message : "Tidak dapat terhubung ke server.");
    }
  }

  async function submitCode(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    if (!code.trim()) {
      setErrors({ code: "Kode wajib diisi." });
      return;
    }
    setErrors({});
    setLoading(true);
    try {
      await api.post("/auth/web/mfa/verify", { mfa_token: mfaToken, code: code.trim() });
      goToApp();
    } catch (err) {
      setLoading(false);
      setCode("");
      if (err instanceof ApiError && err.code === "AUTH_TOKEN_INVALID") {
        // Token sementara 5 menit habis: kembali ke langkah sandi.
        setMfaToken("");
      }
      setFormError(err instanceof ApiError ? err.message : "Tidak dapat terhubung ke server.");
    }
  }

  const alert = formError && (
    <div role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
      {formError}
    </div>
  );

  if (mfaToken) {
    return (
      <form onSubmit={submitCode} noValidate className="space-y-4">
        <div className="flex items-start gap-2 rounded-md bg-primary-50 px-3 py-2 text-sm text-primary-700">
          <ShieldCheck className="mt-0.5 size-4 shrink-0" aria-hidden />
          Masukkan 6 digit kode dari aplikasi authenticator, atau salah satu kode pemulihan.
        </div>
        {alert}
        <Field label="Kode verifikasi" htmlFor="code" error={errors.code}>
          <Input
            id="code"
            inputMode="text"
            autoComplete="one-time-code"
            autoFocus
            value={code}
            onChange={(e) => setCode(e.target.value)}
            invalid={!!errors.code}
            className="font-mono tracking-widest"
          />
        </Field>
        <Button type="submit" loading={loading} className="w-full">
          Verifikasi
        </Button>
        <button
          type="button"
          onClick={() => {
            setMfaToken("");
            setFormError("");
            setCode("");
          }}
          className="w-full text-center text-sm text-slate-500 hover:text-slate-800"
        >
          Kembali
        </button>
      </form>
    );
  }

  return (
    <form onSubmit={submitPassword} noValidate className="space-y-4">
      {alert}
      <Field label="Email" htmlFor="email" error={errors.email}>
        <Input
          id="email"
          type="email"
          autoComplete="username"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          invalid={!!errors.email}
          placeholder="nama@pejuang.id"
        />
      </Field>
      <Field label="Kata sandi" htmlFor="password" error={errors.password}>
        <div className="relative">
          <Input
            id="password"
            type={show ? "text" : "password"}
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            invalid={!!errors.password}
            className="pr-10"
          />
          <button
            type="button"
            onClick={() => setShow((s) => !s)}
            aria-label={show ? "Sembunyikan kata sandi" : "Tampilkan kata sandi"}
            className="absolute inset-y-0 right-0 flex w-10 items-center justify-center text-slate-400 hover:text-slate-600"
          >
            {show ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
          </button>
        </div>
      </Field>
      <Button type="submit" loading={loading} className="w-full">
        Masuk
      </Button>
      {DEMO && (
        <p className="rounded-md bg-slate-50 px-3 py-2 text-xs text-slate-500">
          Mode demo: akun <span className="font-medium text-slate-700">sdm@pejuang.id</span>, kata sandi ada di <code>backend/.env</code>.
        </p>
      )}
    </form>
  );
}
