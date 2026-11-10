"use client";

import { Trash2 } from "lucide-react";
import { useState } from "react";
import { Button, IconButton } from "@/components/ui/button";
import { Field, Input, Select, Textarea, Toggle } from "@/components/ui/form";
import { Badge, ErrorState, Loading } from "@/components/ui/layout";
import { ConfirmDialog, Modal } from "@/components/ui/modal";
import { api } from "@/lib/api-client";
import { DAYS, dateLong, todayWib } from "@/lib/format";
import { useMe, useSave, useWorkArrangements } from "@/lib/hooks";
import { validate, workArrangementSchema } from "@/lib/schemas";
import type { EmployeeListItem, WorkArrangement, WorkArrangementInput } from "@/lib/types";

const MODE = {
  wfa: { label: "WFA", hint: "Boleh presensi dari mana saja. Tercatat wfa dan tidak terikat lokasi kantor." },
  hybrid: { label: "Hybrid", hint: "Di lokasi yang ditugaskan tercatat onsite. Di luar lokasi tetap sah dan tercatat field." },
} as const;

type Mode = keyof typeof MODE;

const emptyForm = () => ({ mode: "wfa" as Mode, flexible_hours: false, starts_on: todayWib(), ends_on: "", weekdays: [] as number[], reason: "" });

function periodLabel(a: WorkArrangement) {
  return `${dateLong(a.starts_on)} sampai ${a.ends_on ? dateLong(a.ends_on) : "tanpa batas"}`;
}

function statusOf(a: WorkArrangement, today: string) {
  if (a.ends_on && a.ends_on < today) return <Badge>Berakhir</Badge>;
  if (a.starts_on > today) return <Badge tone="amber">Terjadwal</Badge>;
  return <Badge tone="green">Berlaku</Badge>;
}

export function EmployeeArrangementsModal({ employee, onClose }: { employee: EmployeeListItem; onClose: () => void }) {
  const me = useMe();
  const list = useWorkArrangements(employee.id);
  // Izin melewati geofence, jadi server menolak bila diberikan untuk diri sendiri.
  const isSelf = me.data?.employee_id === employee.id;
  const [form, setForm] = useState(emptyForm);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [removing, setRemoving] = useState<WorkArrangement | null>(null);
  const path = `/admin/employees/${employee.id}/work-arrangements`;

  const set = <K extends keyof ReturnType<typeof emptyForm>>(k: K, v: ReturnType<typeof emptyForm>[K]) => {
    setForm((f) => ({ ...f, [k]: v }));
    setErrors(({ [k]: _cleared, ...rest }) => rest);
  };

  const save = useSave((v: WorkArrangementInput) => api.post(path, v), {
    success: "Izin kerja ditetapkan.",
    onDone: () => setForm(emptyForm()),
    onFieldErrors: setErrors,
  });
  const remove = useSave((id: string) => api.del(`${path}/${id}`), {
    success: "Izin kerja dicabut.",
    onDone: () => setRemoving(null),
  });

  function toggleDay(day: number) {
    set("weekdays", form.weekdays.includes(day) ? form.weekdays.filter((d) => d !== day) : [...form.weekdays, day]);
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const values: WorkArrangementInput = { ...form, ends_on: form.ends_on || null, reason: form.reason.trim() };
    const found = validate(workArrangementSchema, values) ?? {};
    setErrors(found);
    if (Object.keys(found).length === 0) save.mutate(values);
  }

  const today = todayWib();
  return (
    <>
      <Modal
        open
        size="lg"
        title={`Izin kerja ${employee.full_name}`}
        onClose={onClose}
        footer={
          <>
            <Button variant="secondary" onClick={onClose}>
              Tutup
            </Button>
            {!isSelf && (
              <Button type="submit" form="arrangement-form" loading={save.isPending}>
                Tetapkan izin
              </Button>
            )}
          </>
        }
      >
        <h3 className="text-sm font-medium text-slate-900">Izin yang ada</h3>
        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : list.data.length === 0 ? (
          <p className="mt-1 text-sm text-slate-500">Belum ada izin. Pejuang ini hanya bisa presensi di lokasi yang ditugaskan.</p>
        ) : (
          <ul className="mt-2 divide-y divide-slate-100 rounded-md border border-slate-200">
            {list.data.map((a) => (
              <li key={a.id} className="flex items-start justify-between gap-3 px-3 py-2.5">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <Badge>{MODE[a.mode].label}</Badge>
                    {a.flexible_hours && <Badge>Jam fleksibel</Badge>}
                    {statusOf(a, today)}
                  </div>
                  <p className="mt-1 text-sm text-slate-700">{periodLabel(a)}</p>
                  <p className="text-xs text-slate-500">{a.weekdays.length > 0 ? `Setiap ${a.weekdays.map((d) => DAYS[d - 1]).join(", ")}` : "Setiap hari"}</p>
                  <p className="mt-1 text-sm break-words text-slate-600">{a.reason}</p>
                </div>
                <IconButton label={`Cabut izin ${MODE[a.mode].label}, ${periodLabel(a)}`} onClick={() => setRemoving(a)} className="hover:text-red-600">
                  <Trash2 className="size-4" />
                </IconButton>
              </li>
            ))}
          </ul>
        )}

        {isSelf ? (
          <p className="mt-4 text-sm text-slate-500">Izin kerja untuk akun sendiri harus diberikan peninjau lain.</p>
        ) : (
          <form id="arrangement-form" onSubmit={submit} noValidate className="mt-5 grid grid-cols-1 gap-4 border-t border-slate-100 pt-4 sm:grid-cols-2">
            <h3 className="text-sm font-medium text-slate-900 sm:col-span-2">Tambah izin</h3>
            {!employee.default_shift_name && (
              <p className="rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-800 sm:col-span-2">
                Pejuang ini belum punya shift. Izin tetap bisa disimpan, tetapi presensi ditolak sampai shift ditentukan.
              </p>
            )}
            <Field label="Mode" htmlFor="arrangement_mode" error={errors.mode} hint={MODE[form.mode].hint} required className="sm:col-span-2">
              <Select id="arrangement_mode" value={form.mode} onChange={(e) => set("mode", e.target.value as Mode)} invalid={!!errors.mode}>
                <option value="wfa">{MODE.wfa.label}</option>
                <option value="hybrid">{MODE.hybrid.label}</option>
              </Select>
            </Field>
            <Field label="Tanggal mulai" htmlFor="arrangement_starts_on" error={errors.starts_on} required>
              <Input id="arrangement_starts_on" type="date" value={form.starts_on} onChange={(e) => set("starts_on", e.target.value)} invalid={!!errors.starts_on} />
            </Field>
            <Field label="Tanggal selesai" htmlFor="arrangement_ends_on" error={errors.ends_on} hint="Kosongkan bila tanpa batas.">
              <Input id="arrangement_ends_on" type="date" value={form.ends_on} min={form.starts_on} onChange={(e) => set("ends_on", e.target.value)} invalid={!!errors.ends_on} />
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
              <p className="mt-1 text-xs text-slate-500">Presensi di luar jam shift tetap sah dan tidak dihitung telat.</p>
            </div>
            <Field label="Alasan" htmlFor="arrangement_reason" error={errors.reason} required className="sm:col-span-2">
              <Textarea id="arrangement_reason" rows={2} maxLength={500} value={form.reason} onChange={(e) => set("reason", e.target.value)} invalid={!!errors.reason} />
            </Field>
          </form>
        )}
      </Modal>

      <ConfirmDialog
        open={removing !== null}
        title="Cabut izin kerja?"
        confirmLabel="Cabut"
        message={
          <>
            Izin <strong className="text-slate-800">{removing && MODE[removing.mode].label}</strong> untuk {employee.full_name} langsung berhenti, termasuk untuk presensi pulang hari ini. Presensi yang sudah tercatat tidak berubah.
          </>
        }
        loading={remove.isPending}
        onConfirm={() => removing && remove.mutate(removing.id)}
        onClose={() => setRemoving(null)}
      />
    </>
  );
}
