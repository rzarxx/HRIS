"use client";

import { Pencil, Plus } from "lucide-react";
import { useState } from "react";
import { METHODS, METHOD_ORDER } from "@/components/payroll/methods";
import { Button, IconButton } from "@/components/ui/button";
import { Field, Input, Select, Textarea, Toggle } from "@/components/ui/form";
import { Badge, Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { Modal } from "@/components/ui/modal";
import { Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { rupiah } from "@/lib/format";
import { usePayrollComponents, usePayrollSettings, useSave } from "@/lib/hooks";
import type { PayrollCalcMethod, PayrollComponent } from "@/lib/types";

interface ComponentForm {
  code: string;
  name: string;
  kind: "earning" | "deduction";
  method: PayrollCalcMethod;
  rate: string;
  percent: string;
  divisor: string;
  active: boolean;
  show_on_slip: boolean;
  sort_order: string;
  note: string;
}

const EMPTY: ComponentForm = {
  code: "",
  name: "",
  kind: "earning",
  method: "fixed",
  rate: "",
  percent: "",
  divisor: "",
  active: true,
  show_on_slip: true,
  sort_order: "0",
  note: "",
};

/** Angka dari server ("15000.00") ditampilkan tanpa nol di belakang koma. */
const trimNumber = (v: string | null) => (v == null ? "" : String(Number(v)));

function valueLabel(c: PayrollComponent) {
  const m = METHODS[c.method];
  if (m.percent && c.percent) return `${Number(c.percent).toLocaleString("id-ID")}%`;
  if (c.rate) return rupiah(c.rate);
  if (m.divisor) return `Ubar pokok ÷ ${c.divisor ? Number(c.divisor) : 26}`;
  return "-";
}

function ComponentsCard() {
  const list = usePayrollComponents();
  const [editing, setEditing] = useState<PayrollComponent | "new" | null>(null);
  const [form, setForm] = useState<ComponentForm>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const method = METHODS[form.method];

  const save = useSave(
    (v: ComponentForm) => {
      const body = {
        name: v.name,
        kind: v.kind,
        method: v.method,
        rate: method.rate ? v.rate || null : null,
        percent: method.percent ? v.percent || null : null,
        divisor: method.divisor ? v.divisor || null : null,
        active: v.active,
        show_on_slip: v.show_on_slip,
        sort_order: Number(v.sort_order) || 0,
        note: v.note,
      };
      return editing && editing !== "new"
        ? api.patch(`/admin/payroll/components/${editing.id}`, body)
        : api.post("/admin/payroll/components", { ...body, code: v.code });
    },
    { success: "Komponen ubar tersimpan.", onDone: () => setEditing(null), onFieldErrors: setErrors },
  );

  function open(c: PayrollComponent | "new") {
    setErrors({});
    setForm(
      c === "new"
        ? EMPTY
        : {
            code: c.code,
            name: c.name,
            kind: c.kind,
            method: c.method,
            rate: trimNumber(c.rate),
            percent: trimNumber(c.percent),
            divisor: trimNumber(c.divisor),
            active: c.active,
            show_on_slip: c.show_on_slip,
            sort_order: String(c.sort_order),
            note: c.note,
          },
    );
    setEditing(c);
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    save.mutate(form);
  }

  const set = <K extends keyof ComponentForm>(key: K, value: ComponentForm[K]) => setForm({ ...form, [key]: value });

  return (
    <>
      <Card
        title="Komponen ubar"
        bodyClassName=""
        actions={
          <Button onClick={() => open("new")}>
            <Plus className="size-4" /> Tambah komponen
          </Button>
        }
      >
        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : list.data.length === 0 ? (
          <EmptyState title="Belum ada komponen" description="Tambahkan tunjangan dan potongan. Ubar pokok diambil dari data Pejuang." />
        ) : (
          <Table actions="always">
            <thead>
              <tr>
                <Th>Komponen</Th>
                <Th>Jenis</Th>
                <Th>Cara hitung</Th>
                <Th align="right">Nilai</Th>
                <Th>Status</Th>
                <Th align="right">
                  <span className="sr-only">Aksi</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {list.data.map((c) => (
                <tr key={c.id} className={`hover:bg-slate-50 ${c.active ? "" : "text-slate-500"}`}>
                  <Td>
                    <p className={`font-medium ${c.active ? "text-slate-900" : "text-slate-600"}`}>{c.name}</p>
                    <p className="font-mono text-xs text-slate-500">{c.code}</p>
                  </Td>
                  <Td>{c.kind === "earning" ? <Badge tone="green">Pendapatan</Badge> : <Badge tone="red">Potongan</Badge>}</Td>
                  <Td>{METHODS[c.method].label}</Td>
                  <Td align="right" className="whitespace-nowrap tabular-nums">
                    {valueLabel(c)}
                  </Td>
                  <Td className="whitespace-nowrap">
                    {c.active ? <Badge tone="accent">Aktif</Badge> : <Badge>Nonaktif</Badge>}
                    {!c.show_on_slip && <span className="ml-2 text-xs text-slate-500">Tidak tampil di slip</span>}
                  </Td>
                  <Td align="right">
                    <IconButton label={`Ubah ${c.name}`} onClick={() => open(c)}>
                      <Pencil className="size-4" />
                    </IconButton>
                  </Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card>

      <Modal
        open={editing !== null}
        title={editing === "new" ? "Tambah komponen ubar" : "Ubah komponen ubar"}
        onClose={() => setEditing(null)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setEditing(null)}>
              Batal
            </Button>
            <Button type="submit" form="component-form" loading={save.isPending}>
              Simpan
            </Button>
          </>
        }
      >
        <form id="component-form" onSubmit={submit} noValidate className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Nama" htmlFor="comp-name" error={errors.name} required>
            <Input id="comp-name" value={form.name} onChange={(e) => set("name", e.target.value)} invalid={!!errors.name} placeholder="Uang makan" />
          </Field>
          <Field
            label="Kode"
            htmlFor="comp-code"
            error={errors.code}
            required
            hint={editing === "new" ? "Huruf kapital, angka, garis bawah. Tidak bisa diubah nanti." : "Kode tidak bisa diubah."}
          >
            <Input
              id="comp-code"
              value={form.code}
              onChange={(e) => set("code", e.target.value.toUpperCase().replace(/\s+/g, "_"))}
              invalid={!!errors.code}
              disabled={editing !== "new"}
              placeholder="UANG_MAKAN"
              className="font-mono"
            />
          </Field>
          <Field label="Jenis" htmlFor="comp-kind" error={errors.kind} required>
            <Select id="comp-kind" value={form.kind} onChange={(e) => set("kind", e.target.value as ComponentForm["kind"])}>
              <option value="earning">Pendapatan</option>
              <option value="deduction">Potongan</option>
            </Select>
          </Field>
          <Field label="Cara hitung" htmlFor="comp-method" error={errors.method} required>
            <Select id="comp-method" value={form.method} onChange={(e) => set("method", e.target.value as PayrollCalcMethod)}>
              {METHOD_ORDER.map((m) => (
                <option key={m} value={m}>
                  {METHODS[m].label}
                </option>
              ))}
            </Select>
          </Field>
          <p className="-mt-2 text-xs text-slate-600 sm:col-span-2">{method.hint}</p>
          {method.rate && (
            <Field
              label={`${method.rateLabel} (Rp)`}
              htmlFor="comp-rate"
              error={errors.rate}
              required={method.rate === "required"}
              hint={method.rate === "optional" ? "Kosongkan untuk memakai ubar pokok ÷ pembagi." : undefined}
            >
              <Input id="comp-rate" inputMode="decimal" value={form.rate} onChange={(e) => set("rate", e.target.value)} invalid={!!errors.rate} placeholder="15000" />
            </Field>
          )}
          {method.divisor && (
            <Field label="Pembagi hari" htmlFor="comp-divisor" error={errors.divisor} hint="Bawaan 26 hari bila kosong.">
              <Input id="comp-divisor" inputMode="decimal" value={form.divisor} onChange={(e) => set("divisor", e.target.value)} invalid={!!errors.divisor} placeholder="26" />
            </Field>
          )}
          {method.percent && (
            <Field label="Persen (%)" htmlFor="comp-percent" error={errors.percent} required>
              <Input id="comp-percent" inputMode="decimal" value={form.percent} onChange={(e) => set("percent", e.target.value)} invalid={!!errors.percent} placeholder="2.5" />
            </Field>
          )}
          <Field label="Urutan di slip" htmlFor="comp-order" error={errors.sort_order} hint="Angka kecil tampil lebih dulu.">
            <Input id="comp-order" inputMode="numeric" value={form.sort_order} onChange={(e) => set("sort_order", e.target.value)} invalid={!!errors.sort_order} />
          </Field>
          <Field label="Catatan" htmlFor="comp-note" error={errors.note} className="sm:col-span-2">
            <Textarea id="comp-note" rows={2} value={form.note} onChange={(e) => set("note", e.target.value)} invalid={!!errors.note} />
          </Field>
          <div className="flex flex-wrap gap-x-6 gap-y-2 sm:col-span-2">
            <Toggle checked={form.active} onChange={(v) => set("active", v)}>
              Aktif (dipakai saat hitung ubar)
            </Toggle>
            <Toggle checked={form.show_on_slip} onChange={(v) => set("show_on_slip", v)}>
              Tampil di slip
            </Toggle>
          </div>
        </form>
      </Modal>
    </>
  );
}

function SettingsCard() {
  const settings = usePayrollSettings();
  const [draft, setDraft] = useState<{ cutoff: string; minNetZero: boolean } | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const save = useSave(
    (v: { cutoff: string; minNetZero: boolean }) => api.put("/admin/payroll/settings", { period_cutoff_day: Number(v.cutoff), min_net_zero: v.minNetZero }),
    { success: "Setelan payroll tersimpan.", onDone: () => setDraft(null), onFieldErrors: setErrors },
  );

  if (settings.isPending) return <Card title="Setelan periode">{<Loading />}</Card>;
  if (settings.isError) return <Card title="Setelan periode">{<ErrorState message={settings.error.message} onRetry={() => settings.refetch()} />}</Card>;

  const value = draft ?? { cutoff: String(settings.data.period_cutoff_day), minNetZero: settings.data.min_net_zero };
  const day = Number(value.cutoff);
  const example =
    day === 0 ? "Periode November = 1 sampai 30 November." : day >= 1 && day <= 28 ? `Periode November = ${day + 1} Oktober sampai ${day} November.` : "";

  return (
    <Card
      title="Setelan periode"
      actions={
        <Button onClick={() => save.mutate(value)} loading={save.isPending} disabled={!draft}>
          Simpan
        </Button>
      }
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field label="Tanggal cutoff" htmlFor="cutoff" error={errors.period_cutoff_day} hint={`1 sampai 28, atau 0 untuk akhir bulan. ${example}`}>
          <Input id="cutoff" inputMode="numeric" value={value.cutoff} onChange={(e) => setDraft({ ...value, cutoff: e.target.value })} invalid={!!errors.period_cutoff_day} />
        </Field>
        <div className="sm:pt-7">
          <Toggle checked={value.minNetZero} onChange={(v) => setDraft({ ...value, minNetZero: v })}>
            Ubar bersih tidak pernah minus
          </Toggle>
        </div>
      </div>
    </Card>
  );
}

export default function PayrollComponentsPage() {
  return (
    <>
      <PageHeader
        title="Komponen ubar"
        description="Tunjangan dan potongan yang dihitung otomatis dari presensi. Komponen yang tidak dipakai cukup dinonaktifkan; periode yang sudah dihitung tidak berubah."
      />
      <div className="grid grid-cols-1 gap-6">
        <ComponentsCard />
        <SettingsCard />
      </div>
    </>
  );
}
