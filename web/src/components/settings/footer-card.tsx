"use client";

import { RotateCcw } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/form";
import { Badge, Card, ErrorState, Loading } from "@/components/ui/layout";
import { api } from "@/lib/api-client";
import { dateTime } from "@/lib/format";
import { useBranding, useSave } from "@/lib/hooks";
import type { Branding } from "@/lib/types";

const DEFAULT_TEMPLATE = "© {tahun} PUTI (Pusat Teknologi Informasi) Al Bahjah";
const MAX = 200;

/** Pratinjau sama dengan server: {tahun} diganti tahun berjalan. */
const render = (template: string) => template.replaceAll("{tahun}", String(new Date().getFullYear()));

/** Kartu pengaturan footer web admin dan aplikasi; hanya untuk super admin (settings:manage). */
export function FooterCard() {
  const branding = useBranding();
  const [value, setValue] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (branding.data) setValue(branding.data.footer_template);
  }, [branding.data]);

  const save = useSave((footer_template: string | null) => api.put<{ data: Branding }>("/admin/settings/footer", { footer_template }), {
    success: "Footer disimpan.",
    onFieldErrors: setErrors,
  });

  const trimmed = value.trim();
  const changed = branding.data ? trimmed !== branding.data.footer_template : false;

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (trimmed.length > MAX) {
      setErrors({ footer_template: `Footer maksimal ${MAX} karakter.` });
      return;
    }
    setErrors({});
    save.mutate(trimmed || null);
  }

  return (
    <Card title="Footer website dan aplikasi">
      <p className="mb-4 text-sm text-slate-500">Tampil di bawah setiap halaman web admin, halaman masuk, dan aplikasi Pejuang.</p>
      {branding.isPending ? (
        <Loading />
      ) : branding.isError ? (
        <ErrorState message={branding.error.message} onRetry={() => branding.refetch()} />
      ) : (
        <form onSubmit={submit} noValidate className="flex flex-col gap-4">
          <Field
            label="Teks footer"
            htmlFor="footer_template"
            error={errors.footer_template}
            hint={`Tulis {tahun} untuk tahun yang berganti otomatis. Maksimal ${MAX} karakter, satu baris.`}
          >
            <Input
              id="footer_template"
              value={value}
              maxLength={MAX}
              onChange={(e) => {
                setValue(e.target.value);
                setErrors({});
              }}
              invalid={!!errors.footer_template}
            />
          </Field>
          <div className="rounded-md border border-slate-200 bg-slate-50 px-3 py-3 text-center">
            <p className="mb-1 text-xs font-medium text-slate-500">Pratinjau</p>
            <p className="text-sm text-slate-700">{render(trimmed || DEFAULT_TEMPLATE)}</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button type="submit" loading={save.isPending} disabled={!changed}>
              Simpan footer
            </Button>
            {!branding.data.is_default && (
              <Button type="button" variant="secondary" onClick={() => save.mutate(null)} disabled={save.isPending}>
                <RotateCcw className="size-4" aria-hidden /> Kembalikan bawaan
              </Button>
            )}
            <span className="ml-auto">
              {branding.data.is_default ? (
                <Badge>Teks bawaan</Badge>
              ) : (
                <span className="text-xs text-slate-500">Diubah {dateTime(branding.data.updated_at)}</span>
              )}
            </span>
          </div>
        </form>
      )}
    </Card>
  );
}
