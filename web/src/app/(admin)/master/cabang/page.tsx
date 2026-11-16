"use client";

import { Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { Button, IconButton } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/form";
import { Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog, Modal } from "@/components/ui/modal";
import { Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { number } from "@/lib/format";
import { useBranches, useSave } from "@/lib/hooks";
import { branchSchema, validate, type BranchInput } from "@/lib/schemas";
import type { Branch } from "@/lib/types";

const EMPTY: BranchInput = { code: "", name: "" };

/** Cabang yayasan: masing-masing punya divisi, subdiv, dan atasan sendiri (D53). */
export default function BranchesPage() {
  const list = useBranches();
  const [editing, setEditing] = useState<Branch | "new" | null>(null);
  const [form, setForm] = useState<BranchInput>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [deleting, setDeleting] = useState<Branch | null>(null);

  const save = useSave(
    (v: BranchInput) => (editing && editing !== "new" ? api.patch(`/admin/branches/${editing.id}`, v) : api.post("/admin/branches", v)),
    { success: "Cabang tersimpan.", onDone: () => setEditing(null), onFieldErrors: setErrors },
  );
  const remove = useSave((id: string) => api.del(`/admin/branches/${id}`), {
    success: "Cabang dihapus.",
    onDone: () => setDeleting(null),
  });

  function open(b: Branch | "new") {
    setErrors({});
    setForm(b === "new" ? EMPTY : { code: b.code, name: b.name });
    setEditing(b);
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const fieldErrors = validate(branchSchema, form);
    setErrors(fieldErrors ?? {});
    if (!fieldErrors) save.mutate(form);
  }

  return (
    <>
      <PageHeader
        title="Cabang"
        description="Cabang yayasan. Setiap cabang punya divisi, subdiv, dan atasan sendiri."
        actions={
          <Button onClick={() => open("new")}>
            <Plus className="size-4" /> Tambah cabang
          </Button>
        }
      />
      <Card bodyClassName="">
        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : list.data.length === 0 ? (
          <EmptyState title="Belum ada cabang" description="Tambahkan cabang pertama." />
        ) : (
          <Table actions="always">
            <thead>
              <tr>
                <Th>Kode</Th>
                <Th>Nama cabang</Th>
                <Th align="right">Divisi</Th>
                <Th align="right">Pejuang</Th>
                <Th align="right">
                  <span className="sr-only">Aksi</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {list.data.map((b) => (
                <tr key={b.id} className="hover:bg-slate-50">
                  <Td className="font-mono text-xs">{b.code}</Td>
                  <Td className="font-medium text-slate-900">{b.name}</Td>
                  <Td align="right">{number(b.division_count)}</Td>
                  <Td align="right">{number(b.employee_count)}</Td>
                  <Td align="right" className="whitespace-nowrap">
                    <IconButton label={`Ubah ${b.name}`} onClick={() => open(b)}>
                      <Pencil className="size-4" />
                    </IconButton>
                    <IconButton label={`Hapus ${b.name}`} onClick={() => setDeleting(b)} className="hover:text-red-600">
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
        title={editing === "new" ? "Tambah cabang" : "Ubah cabang"}
        onClose={() => setEditing(null)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setEditing(null)}>
              Batal
            </Button>
            <Button type="submit" form="branch-form" loading={save.isPending}>
              Simpan
            </Button>
          </>
        }
      >
        <form id="branch-form" onSubmit={submit} noValidate className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <Field label="Kode" htmlFor="code" error={errors.code} required>
            <Input id="code" value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} invalid={!!errors.code} placeholder="CRB" />
          </Field>
          <Field label="Nama cabang" htmlFor="name" error={errors.name} required className="sm:col-span-2">
            <Input id="name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} invalid={!!errors.name} placeholder="Cabang Cirebon" />
          </Field>
        </form>
      </Modal>

      <ConfirmDialog
        open={deleting !== null}
        title="Hapus cabang?"
        message={
          <>
            Cabang <strong className="text-slate-800">{deleting?.name}</strong> akan dihapus permanen. Cabang yang masih memiliki divisi tidak dapat dihapus.
          </>
        }
        loading={remove.isPending}
        onConfirm={() => deleting && remove.mutate(deleting.id)}
        onClose={() => setDeleting(null)}
      />
    </>
  );
}
