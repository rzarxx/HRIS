"use client";

import { ArrowDown, ArrowRight, ArrowUp, Plus, Trash2, UserRound } from "lucide-react";
import { useEffect, useState } from "react";
import { Button, IconButton } from "@/components/ui/button";
import { Field, Input, Toggle } from "@/components/ui/form";
import { Badge, Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { Modal } from "@/components/ui/modal";
import { Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { useAdminLeaveTypes, useApprovalFlow, useSave } from "@/lib/hooks";
import type { ApprovalFlowInput, ApproverRole, LeaveQuotaKind, LeaveType, LeaveTypeInput } from "@/lib/types";
import { APPROVER_ROLE, QUOTA_KIND, quotaSummary } from "./request-status";

const ROLES: ApproverRole[] = ["subdiv", "kadiv", "sdm"];
const ROLE_HINT: Record<ApproverRole, string> = {
  subdiv: "Kepala subdiv pemohon",
  kadiv: "Kepala divisi pemohon",
  sdm: "Siapa pun berrole SDM",
};

export function RequestSettings() {
  return (
    <>
      <PageHeader title="Jenis & Alur Pengajuan" description="Atur jenis cuti/izin beserta hak cuti, dan siapa saja yang menyetujui." />
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-5">
        <div className="xl:col-span-3">
          <LeaveTypesCard />
        </div>
        <div className="xl:col-span-2">
          <FlowCard />
        </div>
      </div>
    </>
  );
}

function LeaveTypesCard() {
  const list = useAdminLeaveTypes();
  const [editing, setEditing] = useState<LeaveType | "new" | null>(null);
  return (
    <Card
      title="Jenis cuti & izin"
      bodyClassName=""
      actions={
        <Button size="sm" onClick={() => setEditing("new")}>
          <Plus className="size-4" aria-hidden /> Tambah jenis
        </Button>
      }
    >
      {list.isPending ? (
        <Loading />
      ) : list.isError ? (
        <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
      ) : list.data.length === 0 ? (
        <EmptyState title="Belum ada jenis cuti" />
      ) : (
        <Table actions="always">
          <thead>
            <tr>
              <Th>Jenis</Th>
              <Th>Hak cuti</Th>
              <Th>Ubar</Th>
              <Th>Status</Th>
              <Th>
                <span className="sr-only">Aksi</span>
              </Th>
            </tr>
          </thead>
          <tbody>
            {list.data.map((t) => (
              <tr key={t.id} className={t.is_active ? "hover:bg-slate-50" : "bg-slate-50/60 text-slate-500"}>
                <Td>
                  <p className="font-medium text-slate-900">{t.name}</p>
                  <p className="font-mono text-xs text-slate-500">{t.code}</p>
                </Td>
                <Td>
                  <p>{quotaSummary(t)}</p>
                  <p className="text-xs text-slate-500">{QUOTA_KIND[t.quota_kind].label}</p>
                </Td>
                <Td>{t.paid ? "Tetap dibayar" : "Tidak dibayar"}</Td>
                <Td>{t.is_active ? <Badge tone="green">Aktif</Badge> : <Badge>Nonaktif</Badge>}</Td>
                <Td align="right">
                  <Button variant="secondary" size="sm" onClick={() => setEditing(t)} aria-label={`Ubah ${t.name}`}>
                    Ubah
                  </Button>
                </Td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
      {editing && <LeaveTypeModal value={editing === "new" ? null : editing} onClose={() => setEditing(null)} />}
    </Card>
  );
}

function LeaveTypeModal({ value, onClose }: { value: LeaveType | null; onClose: () => void }) {
  const [form, setForm] = useState({
    code: value?.code ?? "",
    name: value?.name ?? "",
    quota_kind: (value?.quota_kind ?? "none") as LeaveQuotaKind,
    annual_quota_days: value?.annual_quota_days ? String(value.annual_quota_days) : "",
    paid: value?.paid ?? true,
    is_active: value?.is_active ?? true,
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const set = <K extends keyof typeof form>(k: K, v: (typeof form)[K]) => setForm((f) => ({ ...f, [k]: v }));

  const save = useSave(
    (body: LeaveTypeInput) => (value ? api.patch(`/admin/leave-types/${value.id}`, body) : api.post("/admin/leave-types", body)),
    { success: value ? "Jenis cuti diperbarui." : "Jenis cuti ditambahkan.", onDone: onClose, onFieldErrors: setErrors },
  );

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const next: Record<string, string> = {};
    if (!value && !/^[a-z][a-z0-9_]{1,31}$/.test(form.code)) next.code = "2–32 karakter: huruf kecil, angka, atau garis bawah, diawali huruf.";
    if (form.name.trim().length < 2) next.name = "Nama minimal 2 karakter.";
    const days = Number(form.annual_quota_days);
    if (form.quota_kind === "custom" && !(Number.isInteger(days) && days >= 1 && days <= 366)) next.annual_quota_days = "Isi 1 sampai 366 hari.";
    setErrors(next);
    if (Object.keys(next).length) return;
    save.mutate({
      ...(value ? {} : { code: form.code }),
      name: form.name.trim(),
      quota_kind: form.quota_kind,
      annual_quota_days: form.quota_kind === "custom" ? days : null,
      paid: form.paid,
      is_active: form.is_active,
    });
  }

  return (
    <Modal
      open
      title={value ? `Ubah ${value.name}` : "Tambah jenis cuti"}
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" form="leave-type-form" loading={save.isPending}>
            Simpan
          </Button>
        </>
      }
    >
      <form id="leave-type-form" onSubmit={submit} noValidate className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field label="Nama" htmlFor="lt_name" error={errors.name} required>
          <Input id="lt_name" maxLength={60} value={form.name} onChange={(e) => set("name", e.target.value)} invalid={!!errors.name} placeholder="Contoh: Cuti Melahirkan" />
        </Field>
        <Field label="Kode" htmlFor="lt_code" error={errors.code} required={!value} hint={value ? "Kode tidak bisa diubah." : "Dipakai sistem, contoh: maternity."}>
          <Input
            id="lt_code"
            maxLength={32}
            value={form.code}
            disabled={!!value}
            onChange={(e) => set("code", e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, ""))}
            invalid={!!errors.code}
            className="w-full font-mono"
          />
        </Field>
        <fieldset className="sm:col-span-2">
          <legend className="mb-1.5 text-sm font-medium text-slate-700">Hak cuti</legend>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
            {(Object.keys(QUOTA_KIND) as LeaveQuotaKind[]).map((k) => {
              const active = form.quota_kind === k;
              return (
                <label
                  key={k}
                  className={`flex cursor-pointer flex-col gap-1 rounded-lg border p-3 transition-colors ${
                    active ? "border-primary-600 bg-primary-50 ring-1 ring-primary-600" : "border-slate-200 hover:border-slate-300"
                  }`}
                >
                  <span className="flex items-center gap-2 text-sm font-medium text-slate-900">
                    <input type="radio" name="quota_kind" value={k} checked={active} onChange={() => set("quota_kind", k)} className="accent-primary-600" />
                    {QUOTA_KIND[k].label}
                  </span>
                  <span className="text-xs leading-relaxed text-slate-600">{QUOTA_KIND[k].description}</span>
                </label>
              );
            })}
          </div>
        </fieldset>
        {form.quota_kind === "custom" && (
          <Field label="Hak cuti per tahun (hari)" htmlFor="lt_days" error={errors.annual_quota_days} required>
            <Input
              id="lt_days"
              type="number"
              inputMode="numeric"
              min={1}
              max={366}
              value={form.annual_quota_days}
              onChange={(e) => set("annual_quota_days", e.target.value)}
              invalid={!!errors.annual_quota_days}
            />
          </Field>
        )}
        <div className="flex flex-col gap-3 sm:col-span-2">
          <Toggle checked={form.paid} onChange={(v) => set("paid", v)}>
            Ubar tetap dibayar selama cuti ini
          </Toggle>
          <Toggle checked={form.is_active} onChange={(v) => set("is_active", v)}>
            Aktif, bisa dipilih Pejuang saat mengajukan
          </Toggle>
        </div>
      </form>
    </Modal>
  );
}

function FlowCard() {
  const flow = useApprovalFlow("leave");
  const [steps, setSteps] = useState<ApproverRole[]>([]);
  const saved = flow.data?.steps.map((s) => s.approver_role) ?? [];
  useEffect(() => {
    if (flow.data) setSteps(flow.data.steps.map((s) => s.approver_role));
  }, [flow.data]);
  const dirty = steps.join() !== saved.join();
  const unused = ROLES.filter((r) => !steps.includes(r));

  const save = useSave((body: ApprovalFlowInput) => api.put("/admin/approval-flows/leave", body), { success: "Alur persetujuan cuti disimpan." });

  function move(i: number, d: -1 | 1) {
    setSteps((s) => {
      const next = [...s];
      [next[i], next[i + d]] = [next[i + d], next[i]];
      return next;
    });
  }

  return (
    <Card title="Alur persetujuan cuti & izin">
      {flow.isPending ? (
        <Loading />
      ) : flow.isError ? (
        <ErrorState message={flow.error.message} onRetry={() => flow.refetch()} />
      ) : (
        <>
          <div className="mb-4 flex flex-wrap items-center gap-1.5 text-sm" aria-label="Pratinjau alur">
            <span className="inline-flex items-center gap-1 rounded-full bg-slate-100 px-2.5 py-1 text-slate-700">
              <UserRound className="size-3.5" aria-hidden /> Pemohon
            </span>
            {steps.map((r) => (
              <span key={r} className="inline-flex items-center gap-1.5">
                <ArrowRight className="size-3.5 text-slate-400" aria-hidden />
                <span className="rounded-full bg-primary-50 px-2.5 py-1 font-medium text-primary-700">{APPROVER_ROLE[r]}</span>
              </span>
            ))}
          </div>

          <ol className="space-y-2">
            {steps.map((r, i) => (
              <li key={r} className="flex items-center gap-3 rounded-lg border border-slate-200 px-3 py-2">
                <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-slate-100 text-xs font-semibold text-slate-700">{i + 1}</span>
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium text-slate-900">{APPROVER_ROLE[r]}</p>
                  <p className="text-xs text-slate-500">{ROLE_HINT[r]}</p>
                </div>
                <IconButton label={`Naikkan ${APPROVER_ROLE[r]}`} disabled={i === 0} onClick={() => move(i, -1)}>
                  <ArrowUp className="size-4" />
                </IconButton>
                <IconButton label={`Turunkan ${APPROVER_ROLE[r]}`} disabled={i === steps.length - 1} onClick={() => move(i, 1)}>
                  <ArrowDown className="size-4" />
                </IconButton>
                <IconButton label={`Hapus langkah ${APPROVER_ROLE[r]}`} disabled={steps.length === 1} onClick={() => setSteps((s) => s.filter((x) => x !== r))}>
                  <Trash2 className="size-4" />
                </IconButton>
              </li>
            ))}
          </ol>

          {unused.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-2">
              {unused.map((r) => (
                <Button key={r} variant="secondary" size="sm" onClick={() => setSteps((s) => [...s, r])}>
                  <Plus className="size-3.5" aria-hidden /> {APPROVER_ROLE[r]}
                </Button>
              ))}
            </div>
          )}

          <ul className="mt-4 list-disc space-y-1 pl-5 text-xs leading-relaxed text-slate-500">
            <li>Langkah dilewati otomatis bila kepalanya belum ditunjuk atau kepala itu sendiri yang mengajukan.</li>
            <li>Bila semua langkah terlewati, pengajuan tetap diputuskan SDM.</li>
            <li>Perubahan berlaku untuk pengajuan baru; pengajuan yang sedang berjalan memakai alur lamanya.</li>
          </ul>

          <div className="mt-4 flex justify-end gap-2 border-t border-slate-100 pt-4">
            <Button variant="secondary" disabled={!dirty || save.isPending} onClick={() => setSteps(saved)}>
              Batalkan perubahan
            </Button>
            <Button disabled={!dirty} loading={save.isPending} onClick={() => save.mutate({ steps: steps.map((approver_role) => ({ approver_role })) })}>
              Simpan alur
            </Button>
          </div>
        </>
      )}
    </Card>
  );
}
