"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input, Textarea } from "@/components/ui/form";
import { Badge, Card, ErrorState, Loading } from "@/components/ui/layout";
import { api } from "@/lib/api-client";
import { dateTime } from "@/lib/format";
import { useAppVersion, useSave } from "@/lib/hooks";
import type { AppVersion, AppVersionInput } from "@/lib/types";

const VERSION = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;
const MAX_NOTES = 500;

/** Kode versi sama dengan workflow rilis aplikasi: major·1.000.000 + minor·1.000 + patch (mis. 0.14.0 → 14000). */
export function versionCode(name: string): number | null {
  const m = VERSION.exec(name.trim());
  if (!m) return null;
  const [major, minor, patch] = m.slice(1).map(Number);
  if (minor > 999 || patch > 999 || major > 2099) return null;
  return major * 1_000_000 + minor * 1_000 + patch;
}

const versionName = (code: number) => (code > 0 ? `${Math.floor(code / 1_000_000)}.${Math.floor(code / 1_000) % 1_000}.${code % 1_000}` : "");

type Form = { latest: string; min: string; store_url: string; notes: string };

const fromServer = (v: AppVersion): Form => ({
  latest: v.latest_name || versionName(v.latest_code),
  min: versionName(v.min_code),
  store_url: v.store_url,
  notes: v.notes,
});

/**
 * Versi aplikasi Android (D63), khusus super admin. Versi terbaru memunculkan pemberitahuan "Versi baru tersedia" di
 * aplikasi; versi minimum membuat aplikasi yang lebih lama wajib diperbarui sebelum bisa dipakai.
 */
export function AppVersionCard() {
  const current = useAppVersion();
  const [form, setForm] = useState<Form>({ latest: "", min: "", store_url: "", notes: "" });
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (current.data) setForm(fromServer(current.data));
  }, [current.data]);

  const save = useSave((input: AppVersionInput) => api.put<{ data: AppVersion }>("/admin/settings/app-version", input), {
    success: "Versi aplikasi disimpan.",
    onFieldErrors: (f) => setErrors({ ...f, latest: f.latest_name ?? f.latest_code, min: f.min_code }),
  });

  const set = (key: keyof Form, value: string) => {
    setForm((f) => ({ ...f, [key]: value }));
    setErrors({});
  };

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const next: Record<string, string> = {};
    const latest = form.latest.trim() ? versionCode(form.latest) : 0;
    const min = form.min.trim() ? versionCode(form.min) : 0;
    if (latest === null) next.latest = "Tulis versi berbentuk x.y.z, mis. 0.14.0.";
    if (min === null) next.min = "Tulis versi berbentuk x.y.z, mis. 0.13.0.";
    if (latest !== null && min !== null && min > latest) next.min = "Versi minimum tidak boleh lebih tinggi dari versi terbaru.";
    if (form.store_url.trim() && !form.store_url.trim().startsWith("https://")) next.store_url = "Tautan harus diawali https://.";
    if (form.notes.length > MAX_NOTES) next.notes = `Catatan maksimal ${MAX_NOTES} karakter.`;
    if (Object.keys(next).length > 0 || latest === null || min === null) {
      setErrors(next);
      return;
    }
    save.mutate({
      latest_code: latest,
      latest_name: latest > 0 ? form.latest.trim() : "",
      min_code: min,
      store_url: form.store_url.trim(),
      notes: form.notes,
    });
  }

  const raisesMin = current.data ? (versionCode(form.min) ?? 0) > current.data.min_code : false;

  return (
    <Card title="Versi aplikasi Android">
      <p className="mb-4 text-sm text-slate-500">
        Isi setelah rilis baru terbit di Play Store. Aplikasi yang lebih lama menampilkan pemberitahuan <b>Versi baru tersedia</b>; aplikasi di bawah versi
        minimum wajib diperbarui sebelum bisa dipakai.
      </p>
      {current.isPending ? (
        <Loading />
      ) : current.isError ? (
        <ErrorState message={current.error.message} onRetry={() => current.refetch()} />
      ) : (
        <form onSubmit={submit} noValidate className="flex flex-col gap-4">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Versi terbaru" htmlFor="app_latest" error={errors.latest} hint="Contoh 0.14.0. Kosongkan bila belum ada rilis.">
              <Input id="app_latest" value={form.latest} inputMode="decimal" placeholder="0.14.0" onChange={(e) => set("latest", e.target.value)} invalid={!!errors.latest} />
            </Field>
            <Field label="Versi minimum" htmlFor="app_min" error={errors.min} hint="Kosongkan agar semua versi tetap bisa dipakai.">
              <Input id="app_min" value={form.min} inputMode="decimal" placeholder="0.13.0" onChange={(e) => set("min", e.target.value)} invalid={!!errors.min} />
            </Field>
          </div>
          <Field label="Tautan pembaruan" htmlFor="app_store_url" error={errors.store_url} hint="Kosongkan untuk memakai halaman Play Store.">
            <Input id="app_store_url" value={form.store_url} type="url" placeholder="https://play.google.com/…" onChange={(e) => set("store_url", e.target.value)} invalid={!!errors.store_url} />
          </Field>
          <Field label="Catatan rilis" htmlFor="app_notes" error={errors.notes} hint={`Tampil di pemberitahuan aplikasi. Maksimal ${MAX_NOTES} karakter.`}>
            <Textarea id="app_notes" rows={3} value={form.notes} maxLength={MAX_NOTES} onChange={(e) => set("notes", e.target.value)} invalid={!!errors.notes} />
          </Field>
          {raisesMin && (
            <p className="rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-800">
              Menaikkan versi minimum membuat Pejuang dengan aplikasi lama tidak bisa presensi sampai memperbarui. Pastikan versi ini sudah tersedia di Play Store.
            </p>
          )}
          <div className="flex flex-wrap items-center gap-2">
            <Button type="submit" loading={save.isPending}>
              Simpan versi
            </Button>
            <span className="ml-auto">
              {current.data.updated_at ? (
                <span className="text-xs text-slate-500">Diubah {dateTime(current.data.updated_at)}</span>
              ) : (
                <Badge>Belum diatur</Badge>
              )}
            </span>
          </div>
        </form>
      )}
    </Card>
  );
}
