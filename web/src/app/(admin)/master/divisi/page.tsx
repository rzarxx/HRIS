"use client";

import { Pencil, Plus, Search, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { Button, IconButton } from "@/components/ui/button";
import { Field, Input, Select } from "@/components/ui/form";
import { Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog, Modal } from "@/components/ui/modal";
import { Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { number } from "@/lib/format";
import { useAllEmployees, useBranches, useDivisions, useSave } from "@/lib/hooks";
import { divisionSchema, validate, type DivisionInput } from "@/lib/schemas";
import type { DivisionWithStats } from "@/lib/types";

const EMPTY: DivisionInput = { branch_id: "", code: "", name: "", head_employee_id: null };

export default function DivisionsPage() {
  const list = useDivisions();
  const branches = useBranches();
  const multiBranch = (branches.data?.length ?? 0) > 1;
  const employees = useAllEmployees();
  const [q, setQ] = useState("");
  const [editing, setEditing] = useState<DivisionWithStats | "new" | null>(null);
  const [form, setForm] = useState<DivisionInput>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [deleting, setDeleting] = useState<DivisionWithStats | null>(null);

  const rows = useMemo(() => {
    const term = q.trim().toLowerCase();
    return (list.data ?? []).filter((d) => !term || d.name.toLowerCase().includes(term) || d.code.toLowerCase().includes(term) || d.branch_name.toLowerCase().includes(term));
  }, [list.data, q]);

  const save = useSave(
    (v: DivisionInput) => (editing && editing !== "new" ? api.patch(`/admin/divisions/${editing.id}`, v) : api.post("/admin/divisions", v)),
    { success: "Divisi tersimpan.", onDone: () => setEditing(null), onFieldErrors: setErrors },
  );
  const remove = useSave((id: string) => api.del(`/admin/divisions/${id}`), {
    success: "Divisi dihapus.",
    onDone: () => setDeleting(null),
  });

  function open(d: DivisionWithStats | "new") {
    setErrors({});
    // Satu cabang saja: langsung terpilih agar SDM tidak perlu memilih.
    const onlyBranch = branches.data?.length === 1 ? branches.data[0].id : "";
    setForm(d === "new" ? { ...EMPTY, branch_id: onlyBranch } : { branch_id: d.branch_id, code: d.code, name: d.name, head_employee_id: d.head_employee_id });
    setEditing(d);
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const fieldErrors = validate(divisionSchema, form);
    setErrors(fieldErrors ?? {});
    if (!fieldErrors) save.mutate(form);
  }

  const headOptions = (employees.data ?? []).filter((e) => editing === "new" || !editing || e.division_id === editing.id);

  return (
    <>
      <PageHeader
        title="Divisi"
        description="Struktur organisasi tingkat divisi. Kode divisi unik per cabang."
        actions={
          <Button onClick={() => open("new")}>
            <Plus className="size-4" /> Tambah divisi
          </Button>
        }
      />
      <Card bodyClassName="">
        <div className="border-b border-slate-100 p-4">
          <div className="relative max-w-xs">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden />
            <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Cari kode atau nama..." aria-label="Cari divisi" className="pl-9" />
          </div>
        </div>
        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : rows.length === 0 ? (
          <EmptyState title="Divisi tidak ditemukan" description={q ? "Coba kata kunci lain." : "Tambahkan divisi pertama."} />
        ) : (
          <Table actions="always">
            <thead>
              <tr>
                <Th>Kode</Th>
                <Th>Nama divisi</Th>
                {multiBranch && <Th>Cabang</Th>}
                <Th>Kepala divisi</Th>
                <Th align="right">Subdiv</Th>
                <Th align="right">Pejuang</Th>
                <Th align="right">
                  <span className="sr-only">Aksi</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {rows.map((d) => (
                <tr key={d.id} className="hover:bg-slate-50">
                  <Td className="font-mono text-xs">{d.code}</Td>
                  <Td className="font-medium text-slate-900">{d.name}</Td>
                  {multiBranch && <Td>{d.branch_name}</Td>}
                  <Td>{d.head_name ?? <span className="text-slate-400">Belum ditentukan</span>}</Td>
                  <Td align="right">{number(d.subdivision_count)}</Td>
                  <Td align="right">{number(d.employee_count)}</Td>
                  <Td align="right" className="whitespace-nowrap">
                    <IconButton label={`Ubah ${d.name}`} onClick={() => open(d)}>
                      <Pencil className="size-4" />
                    </IconButton>
                    <IconButton label={`Hapus ${d.name}`} onClick={() => setDeleting(d)} className="hover:text-red-600">
                      <Trash2 className="size-4" />
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
        title={editing === "new" ? "Tambah divisi" : "Ubah divisi"}
        onClose={() => setEditing(null)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setEditing(null)}>
              Batal
            </Button>
            <Button type="submit" form="division-form" loading={save.isPending}>
              Simpan
            </Button>
          </>
        }
      >
        <form id="division-form" onSubmit={submit} noValidate className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <Field label="Cabang" htmlFor="branch_id" error={errors.branch_id} required className="sm:col-span-3">
            <Select id="branch_id" value={form.branch_id} onChange={(e) => setForm({ ...form, branch_id: e.target.value })} invalid={!!errors.branch_id}>
              <option value="">— Pilih cabang —</option>
              {branches.data?.map((b) => (
                <option key={b.id} value={b.id}>
                  {b.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Kode" htmlFor="code" error={errors.code} required>
            <Input id="code" value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} invalid={!!errors.code} placeholder="OPS" />
          </Field>
          <Field label="Nama divisi" htmlFor="name" error={errors.name} required className="sm:col-span-2">
            <Input id="name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} invalid={!!errors.name} placeholder="Operasional" />
          </Field>
          <Field label="Kepala divisi" htmlFor="head" hint="Opsional. Hanya Pejuang di divisi ini." className="sm:col-span-3">
            <Select
              id="head"
              value={form.head_employee_id ?? ""}
              onChange={(e) => setForm({ ...form, head_employee_id: e.target.value || null })}
            >
              <option value="">— Belum ditentukan —</option>
              {headOptions.map((e) => (
                <option key={e.id} value={e.id}>
                  {e.full_name} · {e.subdivision_name}
                </option>
              ))}
            </Select>
          </Field>
        </form>
      </Modal>

      <ConfirmDialog
        open={deleting !== null}
        title="Hapus divisi?"
        message={
          <>
            Divisi <strong className="text-slate-800">{deleting?.name}</strong> akan dihapus permanen. Divisi yang masih memiliki subdiv tidak dapat dihapus.
          </>
        }
        loading={remove.isPending}
        onConfirm={() => deleting && remove.mutate(deleting.id)}
        onClose={() => setDeleting(null)}
      />
    </>
  );
}
