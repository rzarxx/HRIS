"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { downloadBlob } from "@/lib/download";
import type { AnnouncementAttachment } from "@/lib/types";
import { PdfPreview } from "./pdf-preview";

/** Private files always pass through the authenticated API before a local preview is created. */
export function AttachmentPreview({ announcementId, attachment }: { announcementId: string; attachment: AnnouncementAttachment }) {
  const [result, setResult] = useState<{ url?: string; blob?: Blob; error?: string }>({});
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    let active = true;
    let url: string | undefined;
    setResult({});
    api.blob(`/announcements/${announcementId}/attachments/${attachment.id}`).then((blob) => {
      if (!active) return;
      url = URL.createObjectURL(blob);
      setResult({ url, blob });
    }).catch(() => {
      if (active) setResult({ error: "Lampiran belum bisa ditampilkan. Silakan coba lagi." });
    });
    return () => { active = false; if (url) URL.revokeObjectURL(url); };
  }, [announcementId, attachment.id, retry]);

  return (
    <div className="mt-2 space-y-1 rounded-md border border-slate-200 bg-white p-2">
      {result.error ? (
        <p role="alert" className="text-xs text-red-700">{result.error} <button type="button" className="underline" onClick={() => setRetry((n) => n + 1)}>Coba lagi</button></p>
      ) : !result.url ? (
        <p role="status" className="py-8 text-center text-xs text-slate-500">Memuat lampiran…</p>
      ) : attachment.content_type === "application/pdf" ? (
        result.blob && <PdfPreview blob={result.blob} name={attachment.file_name} />
      ) : (
        // A blob URL cannot use Next's server image optimizer; the API already optimizes this image.
        // eslint-disable-next-line @next/next/no-img-element
        <img src={result.url} alt={attachment.file_name} className="h-auto w-full rounded" onError={() => setResult({ error: "Gambar belum bisa ditampilkan. Silakan coba lagi." })} />
      )}
      <div className="flex items-center justify-between gap-2 text-[10px] text-slate-500">
        <span className="min-w-0 truncate">{attachment.file_name}</span>
        {result.blob && <button type="button" className="shrink-0 text-primary-700 underline" onClick={() => result.blob && downloadBlob(result.blob, attachment.file_name)}>Unduh</button>}
      </div>
    </div>
  );
}
