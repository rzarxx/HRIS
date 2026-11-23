"use client";

import { CircleCheck } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/form";
import { PasswordInput } from "@/components/ui/password-input";
import { api, ApiError } from "@/lib/api-client";

const EMPTY = { password: "", confirm: "" };

export function ResetForm() {
  // null = belum dibaca dari URL, "" = tautan tidak memuat token.
  const [token, setToken] = useState<string | null>(null);
  const [form, setForm] = useState(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState("");
  const [invalid, setInvalid] = useState(false);
  const [done, setDone] = useState(false);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const value = new URLSearchParams(window.location.hash.slice(1)).get("token") ?? "";
    // Hapus token dari bilah alamat dan riwayat begitu dibaca.
    if (window.location.hash) window.history.replaceState(null, "", window.location.pathname);
    setToken(value);
  }, []);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    const next: Record<string, string> = {};
    if (form.password.length < 10) next.password = "Kata sandi minimal 10 karakter.";
    if (form.password !== form.confirm) next.confirm = "Konfirmasi tidak sama.";
    setErrors(next);
    if (Object.keys(next).length > 0) return;
    setLoading(true);
    try {
      await api.post("/auth/password/reset", { token, password: form.password });
      setForm(EMPTY);
      setDone(true);
    } catch (err) {
      if (err instanceof ApiError && err.code === "RESET_TOKEN_INVALID") {
        setInvalid(true);
      } else {
        if (err instanceof ApiError && err.fields) setErrors(err.fields);
        setFormError(err instanceof ApiError ? err.message : "Belum terhubung ke server. Silakan periksa koneksi Anda.");
      }
    } finally {
      setLoading(false);
    }
  }

  if (token === null) return null;

  if (done) {
    return (
      <div className="space-y-4">
        <div role="status" className="flex items-start gap-2 rounded-md bg-primary-50 px-3 py-2 text-sm text-primary-700">
          <CircleCheck className="mt-0.5 size-4 shrink-0" aria-hidden />
          Kata sandi baru tersimpan. Silakan masuk kembali di web atau aplikasi.
        </div>
        <Link href="/login/" className="block text-center text-sm font-medium text-primary-700 hover:underline">
          Ke halaman masuk
        </Link>
      </div>
    );
  }

  if (!token || invalid) {
    return (
      <div className="space-y-4">
        <div role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          Tautan sudah tidak berlaku. Tautan hanya bisa dipakai sekali dan berlaku 1 jam. Silakan minta tautan baru.
        </div>
        <Link href="/lupa-kata-sandi/" className="block text-center text-sm font-medium text-primary-700 hover:underline">
          Minta tautan baru
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
        <PasswordInput
          id="password"
          autoComplete="new-password"
          autoFocus
          value={form.password}
          onChange={(e) => setForm({ ...form, password: e.target.value })}
          invalid={!!errors.password}
        />
      </Field>
      <Field label="Ulangi kata sandi baru" htmlFor="confirm" error={errors.confirm}>
        <PasswordInput
          id="confirm"
          autoComplete="new-password"
          value={form.confirm}
          onChange={(e) => setForm({ ...form, confirm: e.target.value })}
          invalid={!!errors.confirm}
        />
      </Field>
      <Button type="submit" loading={loading} className="w-full">
        Simpan kata sandi baru
      </Button>
    </form>
  );
}
