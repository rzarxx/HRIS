"use client";

import { FileText, ImageIcon, Paperclip, Trash2 } from "lucide-react";
import { useRef } from "react";
import { Button, IconButton } from "@/components/ui/button";
import { api } from "@/lib/api-client";
import { downloadBlob } from "@/lib/download";
import type { AnnouncementAttachment } from "@/lib/types";

export const MAX_ATTACHMENTS = 5;
const MAX_BYTES = 5 * 1024 * 1024;
const TYPES = ["application/pdf", "image/jpeg", "image/png"];

export function fileSize(bytes: number) {
  return bytes >= 1024 * 1024 ? `${(bytes / 1024 / 1024).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1024))} KB`;
}

/** Memeriksa berkas sebelum diunggah; server memeriksa ulang dari isi berkasnya. */
export function attachmentProblem(file: File, count: number): string | null {
  if (count >= MAX_ATTACHMENTS) return `Lampiran paling banyak ${MAX_ATTACHMENTS} berkas.`;
  if (!TYPES.includes(file.type)) return "Mohon gunakan berkas PDF, JPEG, atau PNG.";
  if (file.size > MAX_BYTES) return "Ukuran berkas paling besar 5 MB.";
  return null;
}

/** Mengunggah satu lampiran ke pengumuman yang sudah tersimpan. */
export function uploadAttachment(announcementId: string, file: File) {
  return api.upload<{ data: { attachments: AnnouncementAttachment[] } }>(`/admin/announcements/${announcementId}/attachments`, file, { name: file.name });
}

type Item = { key: string; name: string; type: string; size: number; saved?: AnnouncementAttachment };

/**
 * Daftar lampiran di form pengumuman. Untuk pengumuman baru, berkas ditampung dulu ([pending]) lalu
 * diunggah setelah pengumuman tersimpan; untuk pengumuman lama, berkas langsung diunggah.
 */
export function AttachmentsField({
  announcementId,
  saved,
  pending,
  busy,
  error,
  onAdd,
  onRemoveSaved,
  onRemovePending,
}: {
  announcementId?: string;
  saved: AnnouncementAttachment[];
  pending: File[];
  busy: boolean;
  error?: string;
  onAdd: (file: File) => void;
  onRemoveSaved: (a: AnnouncementAttachment) => void;
  onRemovePending: (index: number) => void;
}) {
  const input = useRef<HTMLInputElement>(null);
  const items: Item[] = [
    ...saved.map((a) => ({ key: a.id, name: a.file_name, type: a.content_type, size: a.size_bytes, saved: a })),
    ...pending.map((f, i) => ({ key: `p${i}`, name: f.name, type: f.type, size: f.size })),
  ];

  async function download(a: AnnouncementAttachment) {
    if (!announcementId) return;
    downloadBlob(await api.blob(`/announcements/${announcementId}/attachments/${a.id}`), a.file_name);
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <p className="text-sm font-medium text-slate-700">Lampiran</p>
        <Button
          type="button"
          variant="secondary"
          size="sm"
          loading={busy}
          disabled={items.length >= MAX_ATTACHMENTS}
          onClick={() => input.current?.click()}
        >
          <Paperclip className="size-4" aria-hidden /> Tambah lampiran
        </Button>
      </div>
      <input
        ref={input}
        type="file"
        accept="application/pdf,image/jpeg,image/png"
        className="hidden"
        aria-label="Pilih berkas lampiran"
        onChange={(e) => {
          const file = e.target.files?.[0];
          e.target.value = "";
          if (file) onAdd(file);
        }}
      />
      {items.length === 0 ? (
        <p className="text-xs text-slate-500">PDF, JPEG, atau PNG, maks 5 MB per berkas, paling banyak {MAX_ATTACHMENTS} berkas.</p>
      ) : (
        <ul className="divide-y divide-slate-100 rounded-md border border-slate-200">
          {items.map((it, i) => (
            <li key={it.key} className="flex items-center gap-2 px-3 py-2 text-sm">
              {it.type === "application/pdf" ? (
                <FileText className="size-4 shrink-0 text-red-600" aria-hidden />
              ) : (
                <ImageIcon className="size-4 shrink-0 text-primary-600" aria-hidden />
              )}
              {it.saved && announcementId ? (
                <button type="button" onClick={() => it.saved && download(it.saved)} className="min-w-0 flex-1 truncate text-left text-primary-700 hover:underline">
                  {it.name}
                </button>
              ) : (
                <span className="min-w-0 flex-1 truncate text-slate-800">{it.name}</span>
              )}
              <span className="shrink-0 text-xs text-slate-500">{it.saved ? fileSize(it.size) : `${fileSize(it.size)} · belum diunggah`}</span>
              <IconButton
                label={`Hapus ${it.name}`}
                className="hover:text-red-600"
                onClick={() => (it.saved ? onRemoveSaved(it.saved) : onRemovePending(i - saved.length))}
              >
                <Trash2 className="size-4" />
              </IconButton>
            </li>
          ))}
        </ul>
      )}
      {error && <p className="text-xs text-red-600">{error}</p>}
    </div>
  );
}
