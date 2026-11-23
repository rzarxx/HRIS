"use client";

import { MailCheck } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/form";
import { api, ApiError } from "@/lib/api-client";

export function ForgotForm() {
  const [identifier, setIdentifier] = useState("");
  const [error, setError] = useState("");
  const [formError, setFormError] = useState("");
  const [sent, setSent] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    if (!identifier.trim()) {
      setError("NIPY, email, atau nomor WA wajib diisi.");
      return;
    }
    setError("");
    setLoading(true);
    try {
      const res = await api.post<{ data: { message: string } }>("/auth/password/forgot", { identifier: identifier.trim() });
      setSent(res.data.message);
    } catch (err) {
      if (err instanceof ApiError && err.fields?.identifier) setError(err.fields.identifier);
      setFormError(err instanceof ApiError ? err.message : "Belum terhubung ke server. Silakan periksa koneksi Anda.");
    } finally {
      setLoading(false);
    }
  }

  if (sent) {
    return (
      <div className="space-y-4">
        <div role="status" className="flex items-start gap-2 rounded-md bg-primary-50 px-3 py-2 text-sm text-primary-700">
          <MailCheck className="mt-0.5 size-4 shrink-0" aria-hidden />
          <span>{sent}</span>
        </div>
        <p className="text-sm text-slate-500">
          Tautan berlaku 1 jam dan hanya bisa dipakai sekali. Periksa juga folder spam. Bila akun belum memiliki email, hubungi SDM untuk
          mengatur ulang kata sandi.
        </p>
        <Link href="/login/" className="block text-center text-sm font-medium text-primary-700 hover:underline">
          Kembali ke halaman masuk
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
      <Field label="NIPY, email, atau nomor WA" htmlFor="identifier" error={error}>
        <Input
          id="identifier"
          type="text"
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          autoFocus
          value={identifier}
          onChange={(e) => setIdentifier(e.target.value)}
          invalid={!!error}
        />
      </Field>
      <Button type="submit" loading={loading} className="w-full">
        Kirim tautan
      </Button>
      <Link href="/login/" className="block text-center text-sm text-slate-500 hover:text-slate-800">
        Kembali ke halaman masuk
      </Link>
    </form>
  );
}
