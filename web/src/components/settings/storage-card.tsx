"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/form";
import { Badge, Card, ErrorState, Loading } from "@/components/ui/layout";
import { PasswordInput } from "@/components/ui/password-input";
import { api, ApiError } from "@/lib/api-client";
import { useSave, useStorageSettings } from "@/lib/hooks";
import type { StorageInput, StorageSettings } from "@/lib/types";

type Form = { endpoint: string; bucket: string; region: string; access_key: string; secret_key: string };
const EMPTY: Form = { endpoint: "", bucket: "", region: "us-east-1", access_key: "", secret_key: "" };

export function StorageCard() {
  const settings = useStorageSettings();
  const data = settings.data;
  const [form, setForm] = useState<Form>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [running, setRunning] = useState(false);
  const [syncError, setSyncError] = useState("");
  const [progress, setProgress] = useState<StorageSettings["migration"] | null>(null);
  const stop = useRef(false);

  useEffect(() => {
    if (data?.configured) {
      const s = data;
      setForm({ endpoint: s.endpoint, bucket: s.bucket, region: s.region, access_key: s.access_key, secret_key: "" });
    }
  }, [data]);
  useEffect(() => () => { stop.current = true; }, []);

  const save = useSave((body: StorageInput) => api.put<{ data: StorageSettings }>("/admin/settings/storage", body), {
    success: "Koneksi S3 teruji dan pengaturan aktif.",
    onDone: () => { setForm((f) => ({ ...f, secret_key: "" })); setSyncError(""); },
    onFieldErrors: setErrors,
  });

  function submit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    save.mutate({ ...form, endpoint: form.endpoint.trim(), bucket: form.bucket.trim(), region: form.region.trim(), access_key: form.access_key.trim() });
  }

  async function sync() {
    stop.current = false;
    setRunning(true);
    setSyncError("");
    setProgress(null);
    try {
      while (!stop.current) {
        const result = await api.post<{ data: StorageSettings }>("/admin/settings/storage/sync");
        setProgress(result.data.migration);
        if (result.data.migration.done) break;
        // Sinkronisasi panjang tetap di bawah pembatasan permintaan per akun.
        await new Promise((resolve) => setTimeout(resolve, 750));
      }
      await settings.refetch();
    } catch (e) {
      setSyncError(e instanceof ApiError ? e.message : "Sinkronisasi terhenti. Coba lagi untuk melanjutkan.");
      setProgress(null);
      await settings.refetch();
    } finally { setRunning(false); }
  }

  const migration = progress ?? data?.migration;
  return (
    <Card title="Penyimpanan S3 (SeaweedFS)" actions={data && (data.active ? <Badge tone="green">Aktif</Badge> : <Badge>Belum aktif</Badge>)}>
      <p className="mb-4 text-sm text-slate-500">Berkas baru memakai S3 setelah uji baca, tulis, dan hapus berhasil. Berkas lama tetap dapat dibaca dari disk selama migrasi. Minta operator server menyiapkan backup bucket S3 sebelum aktivasi dan simpan cadangan disk sampai restore terverifikasi.</p>
      {settings.isPending ? <Loading /> : settings.isError ? <ErrorState message={settings.error.message} onRetry={() => settings.refetch()} /> : (
        <div className="space-y-5">
          <form onSubmit={submit} className="space-y-3">
            <Field label="Endpoint S3" htmlFor="storage_endpoint" error={errors.endpoint} hint="URL HTTPS server SeaweedFS, tanpa path atau nama bucket.">
              <Input id="storage_endpoint" type="url" value={form.endpoint} onChange={(e) => setForm({ ...form, endpoint: e.target.value })} placeholder="https://s3.example.org" autoCapitalize="none" spellCheck={false} required invalid={!!errors.endpoint} />
            </Field>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <Field label="Bucket" htmlFor="storage_bucket" error={errors.bucket}>
                <Input id="storage_bucket" value={form.bucket} onChange={(e) => setForm({ ...form, bucket: e.target.value })} required invalid={!!errors.bucket} />
              </Field>
              <Field label="Region" htmlFor="storage_region" error={errors.region}>
                <Input id="storage_region" value={form.region} onChange={(e) => setForm({ ...form, region: e.target.value })} required invalid={!!errors.region} />
              </Field>
            </div>
            <Field label="Access key" htmlFor="storage_access" error={errors.access_key}>
              <Input id="storage_access" value={form.access_key} onChange={(e) => setForm({ ...form, access_key: e.target.value })} autoComplete="off" required invalid={!!errors.access_key} />
            </Field>
            <Field label="Secret key" htmlFor="storage_secret" error={errors.secret_key} hint={data?.secret_set ? "Kosongkan untuk tetap memakai secret key yang tersimpan." : undefined}>
              <PasswordInput id="storage_secret" value={form.secret_key} onChange={(e) => setForm({ ...form, secret_key: e.target.value })} autoComplete="new-password" placeholder={data?.secret_set ? "Tersimpan" : ""} invalid={!!errors.secret_key} />
            </Field>
            <Button type="submit" loading={save.isPending} disabled={running}>Uji koneksi dan aktifkan S3</Button>
          </form>
          {data?.active && (
            <div className="border-t border-slate-200 pt-4 space-y-2">
              <p className="text-sm font-medium text-slate-700">Migrasi berkas lama</p>
              <p className="text-sm text-slate-500">Diperiksa: {migration?.scanned ?? 0}; tersalin: {migration?.copied ?? 0}. {migration?.done ? "Selesai. Salinan disk belum dihapus." : "Dapat dilanjutkan setelah halaman ditutup atau koneksi terputus."}</p>
              {syncError && <p role="alert" className="text-sm text-red-600">{syncError}</p>}
              <Button type="button" variant="secondary" onClick={sync} loading={running} disabled={save.isPending}>{migration?.done ? "Periksa ulang salinan S3" : "Mulai / lanjutkan sinkronisasi"}</Button>
              {running && <Button type="button" variant="ghost" onClick={() => { stop.current = true; }}>Hentikan setelah batch ini</Button>}
            </div>
          )}
        </div>
      )}
    </Card>
  );
}
