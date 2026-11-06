"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input, Select, Textarea, Toggle } from "@/components/ui/form";
import { Modal } from "@/components/ui/modal";
import { api } from "@/lib/api-client";
import { DAYS, todayWib } from "@/lib/format";
import { useDivisions, useSave, useSubdivisions } from "@/lib/hooks";
import { validate, workArrangementSchema } from "@/lib/schemas";
import type { BulkWorkArrangementInput, BulkWorkArrangementResult } from "@/lib/types";

const MODE = {
  wfa: { label: "WFA", hint: "Boleh absen dari mana saja. Tercatat wfa dan tidak terikat lokasi kantor." },
  hybrid: { label: "Hybrid", hint: "Di lokasi yang ditugaskan tercatat onsite. Di luar lokasi tetap sah dan tercatat field." },
} as const;

const SKIP_REASON: Record<BulkWorkArrangementResult["skipped"][number]["reason"], string> = {
  self: "Akun sendiri, harus diberikan peninjau lain",
  protected: "Super admin, hanya bisa diatur super admin",
  overlap: "Bertabrakan dengan izin lain",
};

type Mode = keyof typeof MODE;
type Unit = "division" | "subdivision";

const emptyForm = () => ({
  unit: "subdivision" as Unit,
  unit_id: "",
  mode: "wfa" as Mode,
  flexible_hours: false,
  starts_on: todayWib(),
  ends_on: "",
  weekdays: [] as number[],
  reason: "",
});

/** Izin yang sama untuk semua Pejuang aktif di satu unit. Server menyimpannya per Pejuang. */
export function BulkArrangementModal({ onClose }: { onClose: () => void }) {
  const divisions = useDivisions();
  const subdivisions = useSubdivisions();
  const [form, setForm] = useState(emptyForm);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [result, setResult] = useState<BulkWorkArrangementResult | null>(null);

  const set = <K extends keyof ReturnType<typeof emptyForm>>(k: K, v: ReturnType<typeof emptyForm>[K]) => {
    setForm((f) => ({ ...f, [k]: v }));
    setErrors(({ [k]: _cleared, ...rest }) => rest);
  };

  const save = useSave((v: BulkWorkArrangementInput) => api.post<{ data: BulkWorkArrangementResult }>("/admin/work-arrangements/bulk", v), {
    onDone: (res) => setResult(res.data),
    onFieldErrors: (f) => setErrors({ ...f, unit_id: f.division_id ?? f.subdivision_id ?? "" }),
  });

  function toggleDay(day: number) {
    set("weekdays", form.weekdays.includes(day) ? form.weekdays.filter((d) => d !== day) : [...form.weekdays, day]);
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const base = { mode: form.mode, flexible_hours: form.flexible_hours, starts_on: form.starts_on, ends_on: form.ends_on || null, weekdays: form.weekdays, reason: form.reason.trim() };
    const found = validate(workArrangementSchema, base) ?? {};
    if (!form.unit_id) found.unit_id = form.unit === "division" ? "Pilih divisi." : "Pilih subdiv.";
    setErrors(found);
    if (Object.keys(found).length > 0) return;
    save.mutate({ ...base, ...(form.unit === "division" ? { division_id: form.unit_id } : { subdivision_id: form.unit_id }) });
  }

  const options = form.unit === "division" ? (divisions.data ?? []) : (subdivisions.data ?? []);

  if (result) {
    return (
      <Modal open title="Hasil izin massal" onClose={onClose} footer={<Button onClick={onClose}>Selesai</Button>}>
        <p className="text-sm text-slate-700">
          <strong className="text-slate-900">{result.created} Pejuang</strong> mendapat izin.
          {result.skipped.length > 0 && <> {result.skipped.length} Pejuang dilewati:</>}
        </p>
        {result.skipped.length > 0 && (
          <ul className="mt-3 divide-y divide-slate-100 rounded-md border border-slate-200">
            {result.skipped.map((s) => (
              <li key={s.employee_id} className="px-3 py-2 text-sm">
                <p className="font-medium text-slate-900">{s.employee_name}</p>
                <p className="text-xs text-slate-500">{SKIP_REASON[s.reason]}</p>
              </li>
            ))}
          </ul>
        )}
      </Modal>
    );
  }

  return (
    <Modal
      open
      size="lg"
      title="Izin kerja massal"
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" form="bulk-arrangement-form" loading={save.isPending}>
            Tetapkan izin
          </Button>
        </>
      }
    >
      <form id="bulk-arrangement-form" onSubmit={submit} noValidate className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <p className="rounded-md bg-slate-50 px-3 py-2 text-sm text-slate-600 sm:col-span-2">
          Izin diberikan ke semua Pejuang aktif di unit yang dipilih, dan tersimpan per Pejuang. Yang izinnya bertabrakan dilewati dan ditampilkan setelah disimpan.
        </p>
        <Field label="Berlaku untuk" htmlFor="bulk_unit" required>
          <Select
            id="bulk_unit"
            value={form.unit}
            onChange={(e) => {
              set("unit", e.target.value as Unit);
              set("unit_id", "");
            }}
          >
            <option value="subdivision">Satu subdiv</option>
            <option value="division">Satu divisi</option>
          </Select>
        </Field>
        <Field label={form.unit === "division" ? "Divisi" : "Subdiv"} htmlFor="bulk_unit_id" error={errors.unit_id} required>
          <Select id="bulk_unit_id" value={form.unit_id} onChange={(e) => set("unit_id", e.target.value)} invalid={!!errors.unit_id}>
            <option value="">Pilih...</option>
            {options.map((o) => (
              <option key={o.id} value={o.id}>
                {o.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Mode" htmlFor="bulk_mode" error={errors.mode} hint={MODE[form.mode].hint} required className="sm:col-span-2">
          <Select id="bulk_mode" value={form.mode} onChange={(e) => set("mode", e.target.value as Mode)} invalid={!!errors.mode}>
            <option value="wfa">{MODE.wfa.label}</option>
            <option value="hybrid">{MODE.hybrid.label}</option>
          </Select>
        </Field>
        <Field label="Tanggal mulai" htmlFor="bulk_starts_on" error={errors.starts_on} required>
          <Input id="bulk_starts_on" type="date" value={form.starts_on} onChange={(e) => set("starts_on", e.target.value)} invalid={!!errors.starts_on} />
        </Field>
        <Field label="Tanggal selesai" htmlFor="bulk_ends_on" error={errors.ends_on} hint="Kosongkan bila tanpa batas.">
          <Input id="bulk_ends_on" type="date" value={form.ends_on} min={form.starts_on} onChange={(e) => set("ends_on", e.target.value)} invalid={!!errors.ends_on} />
        </Field>
        <fieldset className="sm:col-span-2">
          <legend className="mb-1 text-sm font-medium text-slate-700">Hari berlaku</legend>
          <div className="flex flex-wrap gap-x-4 gap-y-2">
            {DAYS.map((label, i) => (
              <Toggle key={label} checked={form.weekdays.includes(i + 1)} onChange={() => toggleDay(i + 1)}>
                {label}
              </Toggle>
            ))}
          </div>
          {errors.weekdays ? <p className="mt-1 text-xs text-red-600">{errors.weekdays}</p> : <p className="mt-1 text-xs text-slate-500">Kosongkan bila berlaku setiap hari.</p>}
        </fieldset>
        <div className="sm:col-span-2">
          <Toggle checked={form.flexible_hours} onChange={(v) => set("flexible_hours", v)} className="font-medium">
            Jam fleksibel
          </Toggle>
          <p className="mt-1 text-xs text-slate-500">Absen di luar jam shift tetap sah dan tidak dihitung telat.</p>
        </div>
        <Field label="Alasan" htmlFor="bulk_reason" error={errors.reason} required className="sm:col-span-2">
          <Textarea id="bulk_reason" rows={2} maxLength={500} value={form.reason} onChange={(e) => set("reason", e.target.value)} invalid={!!errors.reason} />
        </Field>
      </form>
    </Modal>
  );
}
