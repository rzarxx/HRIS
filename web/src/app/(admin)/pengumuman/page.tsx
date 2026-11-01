"use client";

import { Eye, Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { PhonePreview } from "@/components/announcements/phone-preview";
import { AnnouncementStatusBadge } from "@/components/announcements/status";
import { Button, IconButton } from "@/components/ui/button";
import { Field, Input, Select, Textarea } from "@/components/ui/form";
import { Badge, Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog, Modal } from "@/components/ui/modal";
import { Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { dateTime, fromWibInput, toWibInput } from "@/lib/format";
import { useAnnouncements, useDivisions, useSave } from "@/lib/hooks";
import { announcementSchema, validate, type AnnouncementInput } from "@/lib/schemas";
import type { AnnouncementListItem, AnnouncementType } from "@/lib/types";

interface FormState {
  title: string;
  body: string;
  type: AnnouncementType;
  division_id: string;
  starts_at: string; // datetime-local WIB
  ends_at: string;
  is_active: boolean;
}

const blank = (): FormState => ({
  title: "",
  body: "",
  type: "banner",
  division_id: "",
  starts_at: toWibInput(new Date().toISOString()),
  ends_at: "",
  is_active: true,
});

export default function AnnouncementsPage() {
  const list = useAnnouncements();
  const divisions = useDivisions();
  const [editing, setEditing] = useState<AnnouncementListItem | "new" | null>(null);
  const [form, setForm] = useState<FormState>(blank);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [previewing, setPreviewing] = useState<AnnouncementListItem | null>(null);
  const [deleting, setDeleting] = useState<AnnouncementListItem | null>(null);

  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => {
    setForm((f) => ({ ...f, [k]: v }));
    setErrors(({ [k]: _cleared, ...rest }) => rest);
  };

  const save = useSave(
    (v: AnnouncementInput) =>
      editing && editing !== "new" ? api.put(`/admin/announcements/${editing.id}`, v) : api.post("/admin/announcements", v),
    { success: "Pengumuman tersimpan.", onDone: () => setEditing(null), onFieldErrors: setErrors },
  );
  const remove = useSave((id: string) => api.del(`/admin/announcements/${id}`), {
    success: "Pengumuman dihapus.",
    onDone: () => setDeleting(null),
  });

  function open(a: AnnouncementListItem | "new") {
    setErrors({});
    setForm(
      a === "new"
        ? blank()
        : {
            title: a.title,
            body: a.body,
            type: a.type,
            division_id: a.division_id ?? "",
            starts_at: toWibInput(a.starts_at),
            ends_at: toWibInput(a.ends_at),
            is_active: a.is_active,
          },
    );
    setEditing(a);
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const values: AnnouncementInput = {
      title: form.title,
      body: form.body,
      type: form.type,
      division_id: form.division_id || null,
      starts_at: fromWibInput(form.starts_at) ?? "",
      ends_at: fromWibInput(form.ends_at),
      is_active: form.is_active,
    };
    const fieldErrors = validate(announcementSchema, values);
    setErrors(fieldErrors ?? {});
    if (!fieldErrors) save.mutate(values);
  }

  return (
    <>
      <PageHeader
        title="Pengumuman"
        description="Pop-up dan banner yang tampil di aplikasi Pejuang sesuai jadwal dan target divisi."
        actions={
          <Button onClick={() => open("new")}>
            <Plus className="size-4" /> Buat pengumuman
          </Button>
        }
      />
      <Card bodyClassName="">
        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : list.data.length === 0 ? (
          <EmptyState title="Belum ada pengumuman" action={<Button onClick={() => open("new")}>Buat pengumuman</Button>} />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>Judul</Th>
                <Th>Tipe</Th>
                <Th>Target</Th>
                <Th>Jadwal tayang (WIB)</Th>
                <Th>Status</Th>
                <Th align="right">
                  <span className="sr-only">Aksi</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {list.data.map((a) => (
                <tr key={a.id} className="hover:bg-slate-50">
                  <Td>
                    <p className="font-medium text-slate-900">{a.title}</p>
                    <p className="line-clamp-1 max-w-md text-xs text-slate-500">{a.body}</p>
                  </Td>
                  <Td>{a.type === "popup" ? <Badge tone="blue">Pop-up</Badge> : <Badge>Banner</Badge>}</Td>
                  <Td className="whitespace-nowrap">{a.division_name ?? "Semua divisi"}</Td>
                  <Td className="text-xs whitespace-nowrap">
                    {dateTime(a.starts_at)}
                    <br />
                    <span className="text-slate-500">s.d. {a.ends_at ? dateTime(a.ends_at) : "tanpa batas"}</span>
                  </Td>
                  <Td>
                    <AnnouncementStatusBadge a={a} />
                  </Td>
                  <Td align="right" className="whitespace-nowrap">
                    <IconButton label={`Pratinjau ${a.title}`} onClick={() => setPreviewing(a)}>
                      <Eye className="size-4" />
                    </IconButton>
                    <IconButton label={`Ubah ${a.title}`} onClick={() => open(a)}>
                      <Pencil className="size-4" />
                    </IconButton>
                    <IconButton label={`Hapus ${a.title}`} onClick={() => setDeleting(a)} className="hover:text-red-600">
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
        size="lg"
        title={editing === "new" ? "Buat pengumuman" : "Ubah pengumuman"}
        onClose={() => setEditing(null)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setEditing(null)}>
              Batal
            </Button>
            <Button type="submit" form="announcement-form" loading={save.isPending}>
              Simpan
            </Button>
          </>
        }
      >
        <div className="grid gap-6 md:grid-cols-[1fr_auto]">
          <form id="announcement-form" onSubmit={submit} noValidate className="grid content-start gap-4 sm:grid-cols-2">
            <Field label="Judul" htmlFor="title" error={errors.title} required className="sm:col-span-2">
              <Input id="title" value={form.title} onChange={(e) => set("title", e.target.value)} invalid={!!errors.title} />
            </Field>
            <Field label="Isi" htmlFor="body" error={errors.body} required className="sm:col-span-2">
              <Textarea id="body" rows={4} value={form.body} onChange={(e) => set("body", e.target.value)} invalid={!!errors.body} />
            </Field>
            <Field label="Tipe" htmlFor="type" error={errors.type} required hint={form.type === "popup" ? "Tampil sekali sampai ditandai dibaca." : "Tampil di beranda sampai berakhir."}>
              <Select id="type" value={form.type} onChange={(e) => set("type", e.target.value as AnnouncementType)}>
                <option value="banner">Banner</option>
                <option value="popup">Pop-up</option>
              </Select>
            </Field>
            <Field label="Target" htmlFor="division_id">
              <Select id="division_id" value={form.division_id} onChange={(e) => set("division_id", e.target.value)}>
                <option value="">Semua divisi</option>
                {divisions.data?.map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.name}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Mulai tayang (WIB)" htmlFor="starts_at" error={errors.starts_at} required>
              <Input id="starts_at" type="datetime-local" value={form.starts_at} onChange={(e) => set("starts_at", e.target.value)} invalid={!!errors.starts_at} />
            </Field>
            <Field label="Berakhir (WIB)" htmlFor="ends_at" error={errors.ends_at} hint="Kosongkan bila tanpa batas.">
              <Input id="ends_at" type="datetime-local" value={form.ends_at} onChange={(e) => set("ends_at", e.target.value)} invalid={!!errors.ends_at} />
            </Field>
            <label className="flex items-center gap-2 text-sm text-slate-700 sm:col-span-2">
              <input type="checkbox" checked={form.is_active} onChange={(e) => set("is_active", e.target.checked)} className="size-4 rounded border-slate-300 accent-primary-600" />
              Aktif
            </label>
          </form>
          <div>
            <p className="mb-2 text-center text-xs font-medium text-slate-500">Pratinjau di aplikasi</p>
            <PhonePreview title={form.title} body={form.body} type={form.type} />
          </div>
        </div>
      </Modal>

      <Modal open={previewing !== null} title="Pratinjau pengumuman" size="sm" onClose={() => setPreviewing(null)}>
        {previewing && <PhonePreview title={previewing.title} body={previewing.body} type={previewing.type} />}
      </Modal>

      <ConfirmDialog
        open={deleting !== null}
        title="Hapus pengumuman?"
        message={
          <>
            Pengumuman <strong className="text-slate-800">{deleting?.title}</strong> akan dihapus dan tidak tampil lagi di aplikasi.
          </>
        }
        loading={remove.isPending}
        onConfirm={() => deleting && remove.mutate(deleting.id)}
        onClose={() => setDeleting(null)}
      />
    </>
  );
}
