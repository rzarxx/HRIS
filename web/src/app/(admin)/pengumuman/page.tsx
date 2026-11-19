"use client";

import { useQueryClient } from "@tanstack/react-query";
import { Eye, Paperclip, Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { attachmentProblem, AttachmentsField, uploadAttachment } from "@/components/announcements/attachments-field";
import { PhonePreview } from "@/components/announcements/phone-preview";
import { ReadersModal, ReadProgress } from "@/components/announcements/readers";
import { AnnouncementStatusBadge } from "@/components/announcements/status";
import { Button, IconButton } from "@/components/ui/button";
import { Field, Input, Select, Textarea, Toggle } from "@/components/ui/form";
import { Badge, Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog, Modal } from "@/components/ui/modal";
import { Table, Td, Th } from "@/components/ui/table";
import { api, ApiError } from "@/lib/api-client";
import { dateTime, fromWibInput, toWibInput } from "@/lib/format";
import { useAnnouncements, useDivisions, useSave } from "@/lib/hooks";
import { announcementSchema, validate, type AnnouncementInput } from "@/lib/schemas";
import type { AnnouncementAttachment, AnnouncementListItem, AnnouncementType } from "@/lib/types";

interface FormState {
  title: string;
  body: string;
  type: AnnouncementType;
  division_id: string;
  starts_at: string; // datetime-local WIB
  ends_at: string;
  is_active: boolean;
  min_read_seconds: string;
}

const blank = (): FormState => ({
  title: "",
  body: "",
  type: "banner",
  division_id: "",
  starts_at: toWibInput(new Date().toISOString()),
  ends_at: "",
  is_active: true,
  min_read_seconds: "10",
});

export default function AnnouncementsPage() {
  const list = useAnnouncements();
  const divisions = useDivisions();
  const [editing, setEditing] = useState<AnnouncementListItem | "new" | null>(null);
  const [form, setForm] = useState<FormState>(blank);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [previewing, setPreviewing] = useState<AnnouncementListItem | null>(null);
  const [deleting, setDeleting] = useState<AnnouncementListItem | null>(null);
  const [reading, setReading] = useState<AnnouncementListItem | null>(null);
  // Lampiran: yang sudah tersimpan di server, dan yang ditampung untuk pengumuman baru.
  const [savedFiles, setSavedFiles] = useState<AnnouncementAttachment[]>([]);
  const [pendingFiles, setPendingFiles] = useState<File[]>([]);
  const [fileBusy, setFileBusy] = useState(false);
  const [fileError, setFileError] = useState<string>();
  const qc = useQueryClient();
  const editingId = editing && editing !== "new" ? editing.id : undefined;

  async function addFile(file: File) {
    const problem = attachmentProblem(file, savedFiles.length + pendingFiles.length);
    setFileError(problem ?? undefined);
    if (problem) return;
    if (!editingId) {
      setPendingFiles((p) => [...p, file]);
      return;
    }
    setFileBusy(true);
    try {
      const res = await uploadAttachment(editingId, file);
      setSavedFiles(res.data.attachments);
      await qc.invalidateQueries({ queryKey: ["announcements"] });
    } catch (e) {
      setFileError(e instanceof ApiError ? (e.fields?.file ?? e.message) : "Lampiran belum terunggah. Silakan coba lagi.");
    } finally {
      setFileBusy(false);
    }
  }

  async function removeSavedFile(a: AnnouncementAttachment) {
    if (!editingId) return;
    setFileBusy(true);
    try {
      await api.del(`/admin/announcements/${editingId}/attachments/${a.id}`);
      setSavedFiles((list) => list.filter((x) => x.id !== a.id));
      await qc.invalidateQueries({ queryKey: ["announcements"] });
    } catch (e) {
      setFileError(e instanceof ApiError ? e.message : "Lampiran belum terhapus. Silakan coba lagi.");
    } finally {
      setFileBusy(false);
    }
  }

  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => {
    setForm((f) => ({ ...f, [k]: v }));
    setErrors(({ [k]: _cleared, ...rest }) => rest);
  };

  const save = useSave(
    async (v: AnnouncementInput) => {
      if (editingId) return api.patch(`/admin/announcements/${editingId}`, v);
      const created = await api.post<{ data: AnnouncementListItem }>("/admin/announcements", v);
      // Lampiran pengumuman baru diunggah setelah pengumuman tersimpan dan punya ID.
      for (const file of pendingFiles) await uploadAttachment(created.data.id, file);
      return created;
    },
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
            min_read_seconds: String(a.min_read_seconds),
          },
    );
    setSavedFiles(a === "new" ? [] : a.attachments);
    setPendingFiles([]);
    setFileError(undefined);
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
      min_read_seconds: form.min_read_seconds.trim() === "" ? Number.NaN : Number(form.min_read_seconds),
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
          <Table actions="always">
            <thead>
              <tr>
                <Th>Judul</Th>
                <Th>Tipe</Th>
                <Th>Target</Th>
                <Th>Jadwal tayang (WIB)</Th>
                <Th>Status</Th>
                <Th>Dibaca</Th>
                <Th align="right">
                  <span className="sr-only">Aksi</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {list.data.map((a) => (
                <tr key={a.id} className="hover:bg-slate-50">
                  <Td>
                    <p className="font-medium text-slate-900">
                      {a.title}
                      {a.attachments.length > 0 && (
                        <span className="ml-2 inline-flex items-center gap-0.5 text-xs font-normal text-slate-500" title={`${a.attachments.length} lampiran`}>
                          <Paperclip className="size-3" aria-hidden />
                          {a.attachments.length}
                        </span>
                      )}
                    </p>
                    <p className="line-clamp-1 max-w-md text-xs text-slate-500">{a.body}</p>
                  </Td>
                  <Td>{a.type === "popup" ? <Badge tone="accent">Pop-up</Badge> : <Badge>Banner</Badge>}</Td>
                  <Td className="whitespace-nowrap">{a.division_name ?? "Semua divisi"}</Td>
                  <Td className="text-xs whitespace-nowrap">
                    {dateTime(a.starts_at)}
                    <br />
                    <span className="text-slate-500">s.d. {a.ends_at ? dateTime(a.ends_at) : "tanpa batas"}</span>
                  </Td>
                  <Td>
                    <AnnouncementStatusBadge a={a} />
                  </Td>
                  <Td>
                    <ReadProgress a={a} onOpen={() => setReading(a)} />
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
        <div className="grid grid-cols-1 gap-6 md:grid-cols-[minmax(0,1fr)_auto]">
          <form id="announcement-form" onSubmit={submit} noValidate className="grid grid-cols-1 content-start gap-4 sm:grid-cols-2">
            <Field label="Judul" htmlFor="title" error={errors.title} required className="sm:col-span-2">
              <Input id="title" value={form.title} onChange={(e) => set("title", e.target.value)} invalid={!!errors.title} />
            </Field>
            <Field label="Isi" htmlFor="body" error={errors.body} required className="sm:col-span-2">
              <Textarea id="body" rows={4} value={form.body} onChange={(e) => set("body", e.target.value)} invalid={!!errors.body} />
            </Field>
            <Field label="Tipe" htmlFor="type" error={errors.type} required hint={
                form.type === "popup"
                  ? "Muncul saat aplikasi dibuka sampai ditandai dibaca. Bila ada tanggal berakhir, setelah dibaca tetap muncul sekali sehari dengan tombol Tutup sampai berakhir."
                  : "Tampil di beranda sampai berakhir."
              }>
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
            <Field
              label="Jeda sebelum Sudah baca (detik)"
              htmlFor="min_read_seconds"
              error={errors.min_read_seconds}
              hint="0 sampai 120. Tombol Sudah baca di aplikasi aktif setelah hitung mundur ini, dan server menolak yang lebih cepat."
              className="sm:col-span-2"
            >
              <Input
                id="min_read_seconds"
                type="number"
                inputMode="numeric"
                min={0}
                max={120}
                value={form.min_read_seconds}
                onChange={(e) => set("min_read_seconds", e.target.value)}
                invalid={!!errors.min_read_seconds}
                className="sm:max-w-40"
              />
            </Field>
            <div className="sm:col-span-2">
              <AttachmentsField
                announcementId={editingId}
                saved={savedFiles}
                pending={pendingFiles}
                busy={fileBusy}
                error={fileError}
                onAdd={addFile}
                onRemoveSaved={removeSavedFile}
                onRemovePending={(i) => setPendingFiles((p) => p.filter((_, j) => j !== i))}
              />
            </div>
            <Toggle checked={form.is_active} onChange={(v) => set("is_active", v)} className="sm:col-span-2">
              Aktif
            </Toggle>
          </form>
          <div>
            <p className="mb-2 text-center text-xs font-medium text-slate-500">Pratinjau di aplikasi</p>
            <PhonePreview
              title={form.title}
              body={form.body}
              type={form.type}
              minRead={Number(form.min_read_seconds) || 0}
              files={[...savedFiles.map((a) => a.file_name), ...pendingFiles.map((f) => f.name)]}
            />
          </div>
        </div>
      </Modal>

      <Modal open={previewing !== null} title="Pratinjau pengumuman" size="sm" onClose={() => setPreviewing(null)}>
        {previewing && (
          <PhonePreview
            title={previewing.title}
            body={previewing.body}
            type={previewing.type}
            minRead={previewing.min_read_seconds}
            files={previewing.attachments.map((a) => a.file_name)}
          />
        )}
      </Modal>

      <ReadersModal a={reading} onClose={() => setReading(null)} />

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
