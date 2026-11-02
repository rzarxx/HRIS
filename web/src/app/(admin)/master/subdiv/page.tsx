"use client";

import { Pencil, Plus, Search, Trash2 } from "lucide-react";
import Link from "next/link";
import { useMemo, useState } from "react";
import { Button, IconButton } from "@/components/ui/button";
import { Field, Input, Select } from "@/components/ui/form";
import { Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog, Modal } from "@/components/ui/modal";
import { Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { number } from "@/lib/format";
import { useAllEmployees, useDivisions, useSave, useSubdivisions } from "@/lib/hooks";
import { subdivisionSchema, validate, type SubdivisionInput } from "@/lib/schemas";
import type { SubdivisionWithStats } from "@/lib/types";

const EMPTY: SubdivisionInput = { division_id: "", code: "", name: "", head_employee_id: null };

export default function SubdivisionsPage() {
  const list = useSubdivisions();
  const divisions = useDivisions();
  const employees = useAllEmployees();
  const [q, setQ] = useState("");
  const [divisionFilter, setDivisionFilter] = useState("");
  const [editing, setEditing] = useState<SubdivisionWithStats | "new" | null>(null);
  const [form, setForm] = useState<SubdivisionInput>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [deleting, setDeleting] = useState<SubdivisionWithStats | null>(null);

  const rows = useMemo(() => {
    const term = q.trim().toLowerCase();
    return (list.data ?? []).filter(
      (s) =>
        (!divisionFilter || s.division_id === divisionFilter) &&
        (!term || s.name.toLowerCase().includes(term) || s.code.toLowerCase().includes(term)),
    );
  }, [list.data, q, divisionFilter]);

  const save = useSave(
    (v: SubdivisionInput) =>
      editing && editing !== "new" ? api.patch(`/admin/subdivisions/${editing.id}`, v) : api.post("/admin/subdivisions", v),
    { success: "Subdiv tersimpan.", onDone: () => setEditing(null), onFieldErrors: setErrors },
  );
  const remove = useSave((id: string) => api.del(`/admin/subdivisions/${id}`), {
    success: "Subdiv dihapus.",
    onDone: () => setDeleting(null),
  });

  function open(s: SubdivisionWithStats | "new") {
    setErrors({});
    setForm(
      s === "new"
        ? { ...EMPTY, division_id: divisionFilter }
        : { division_id: s.division_id, code: s.code, name: s.name, head_employee_id: s.head_employee_id },
    );
    setEditing(s);
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const fieldErrors = validate(subdivisionSchema, form);
    setErrors(fieldErrors ?? {});
    if (!fieldErrors) save.mutate(form);
  }

  const headOptions = editing && editing !== "new" ? (employees.data ?? []).filter((e) => e.subdivision_id === editing.id) : [];

  return (
    <>
      <PageHeader
        title="Subdiv"
        description="Sub divisi di bawah setiap divisi."
        actions={
          <Button onClick={() => open("new")}>
            <Plus className="size-4" /> Tambah subdiv
          </Button>
        }
      />
      <Card bodyClassName="">
        <div className="flex flex-wrap gap-3 border-b border-slate-100 p-4">
          <div className="relative w-full max-w-xs">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden />
            <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Cari kode atau nama..." aria-label="Cari subdiv" className="pl-9" />
          </div>
          <Select value={divisionFilter} onChange={(e) => setDivisionFilter(e.target.value)} aria-label="Filter divisi" className="w-auto min-w-48">
            <option value="">Semua divisi</option>
            {divisions.data?.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
              </option>
            ))}
          </Select>
        </div>
        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : rows.length === 0 ? (
          <EmptyState title="Subdiv tidak ditemukan" description="Ubah filter atau tambahkan subdiv baru." />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>Kode</Th>
                <Th>Nama subdiv</Th>
                <Th>Divisi</Th>
                <Th>Kepala subdiv</Th>
                <Th align="right">Pejuang</Th>
                <Th align="right">
                  <span className="sr-only">Aksi</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {rows.map((s) => (
                <tr key={s.id} className="hover:bg-slate-50">
                  <Td className="font-mono text-xs">{s.code}</Td>
                  <Td className="font-medium text-slate-900">{s.name}</Td>
                  <Td>{s.division_name}</Td>
                  <Td>{s.head_name ?? <span className="text-slate-400">Belum ditentukan</span>}</Td>
                  <Td align="right">
                    <Link href={`/master/pejuang?subdivision_id=${s.id}`} className="text-primary-600 hover:underline">
                      {number(s.employee_count)}
                    </Link>
                  </Td>
                  <Td align="right" className="whitespace-nowrap">
                    <IconButton label={`Ubah ${s.name}`} onClick={() => open(s)}>
                      <Pencil className="size-4" />
                    </IconButton>
                    <IconButton label={`Hapus ${s.name}`} onClick={() => setDeleting(s)} className="hover:text-red-600">
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
        title={editing === "new" ? "Tambah subdiv" : "Ubah subdiv"}
        onClose={() => setEditing(null)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setEditing(null)}>
              Batal
            </Button>
            <Button type="submit" form="subdiv-form" loading={save.isPending}>
              Simpan
            </Button>
          </>
        }
      >
        <form id="subdiv-form" onSubmit={submit} noValidate className="grid gap-4 sm:grid-cols-3">
          <Field label="Divisi" htmlFor="division" error={errors.division_id} required className="sm:col-span-3">
            <Select
              id="division"
              value={form.division_id}
              onChange={(e) => setForm({ ...form, division_id: e.target.value })}
              invalid={!!errors.division_id}
            >
              <option value="">— Pilih divisi —</option>
              {divisions.data?.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Kode" htmlFor="code" error={errors.code} required>
            <Input id="code" value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} invalid={!!errors.code} placeholder="OPS-LAP" />
          </Field>
          <Field label="Nama subdiv" htmlFor="name" error={errors.name} required className="sm:col-span-2">
            <Input id="name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} invalid={!!errors.name} placeholder="Lapangan" />
          </Field>
          {editing !== "new" && (
            <Field label="Kepala subdiv" htmlFor="head" hint="Hanya Pejuang di subdiv ini." className="sm:col-span-3">
              <Select
                id="head"
                value={form.head_employee_id ?? ""}
                onChange={(e) => setForm({ ...form, head_employee_id: e.target.value || null })}
              >
                <option value="">— Belum ditentukan —</option>
                {headOptions.map((e) => (
                  <option key={e.id} value={e.id}>
                    {e.full_name} · {e.position}
                  </option>
                ))}
              </Select>
            </Field>
          )}
        </form>
      </Modal>

      <ConfirmDialog
        open={deleting !== null}
        title="Hapus subdiv?"
        message={
          <>
            Subdiv <strong className="text-slate-800">{deleting?.name}</strong> akan dihapus permanen. Subdiv yang masih memiliki Pejuang tidak dapat dihapus.
          </>
        }
        loading={remove.isPending}
        onConfirm={() => deleting && remove.mutate(deleting.id)}
        onClose={() => setDeleting(null)}
      />
    </>
  );
}
