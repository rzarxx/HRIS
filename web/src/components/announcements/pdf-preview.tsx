"use client";

import { useEffect, useRef, useState } from "react";
import type { PDFDocumentLoadingTask, PDFDocumentProxy, RenderTask } from "pdfjs-dist";

/** Render pixels only: no PDF scripts, forms, embedded files or external links. */
export function PdfPreview({ blob, name }: { blob: Blob; name: string }) {
  const [doc, setDoc] = useState<PDFDocumentProxy>();
  const [page, setPage] = useState(1);
  const [error, setError] = useState(false);
  const [ready, setReady] = useState(false);
  const canvas = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    let active = true;
    let task: PDFDocumentLoadingTask | undefined;
    setDoc(undefined);
    setPage(1);
    setError(false);
    (async () => {
      const pdf = await import("pdfjs-dist");
      const data = new Uint8Array(await blob.arrayBuffer());
      if (!active) return;
      pdf.GlobalWorkerOptions.workerSrc = "/pdfjs/pdf.worker.min.mjs";
      task = pdf.getDocument({ data, enableXfa: false, useWasm: false,
        cMapUrl: "/pdfjs/cmaps/", cMapPacked: true, standardFontDataUrl: "/pdfjs/standard_fonts/",
        wasmUrl: "/pdfjs/wasm/", iccUrl: "/pdfjs/iccs/", maxImageSize: 24_000_000, canvasMaxAreaInBytes: 16_000_000 });
      const document = await task.promise;
      if (active) setDoc(document);
    })().catch(() => { if (active) setError(true); });
    return () => { active = false; void task?.destroy(); };
  }, [blob]);

  useEffect(() => {
    if (!doc || !canvas.current) return;
    let active = true;
    let render: RenderTask | undefined;
    const target = canvas.current;
    setReady(false);
    (async () => {
      const pdfPage = await doc.getPage(page);
      if (!active) return;
      const bounds = pdfPage.getViewport({ scale: 1 });
      const viewport = pdfPage.getViewport({ scale: Math.min(1000 / bounds.width, 1600 / bounds.height) });
      target.width = Math.max(1, Math.ceil(viewport.width));
      target.height = Math.max(1, Math.ceil(viewport.height));
      render = pdfPage.render({ canvas: target, viewport });
      await render.promise;
      if (active) setReady(true);
    })().catch(() => { if (active) setError(true); });
    return () => { active = false; render?.cancel(); };
  }, [doc, page]);

  if (error) return <p role="alert" className="p-3 text-xs text-slate-600">Pratinjau PDF belum tersedia. Gunakan Unduh untuk membukanya.</p>;
  return (
    <div className="space-y-2">
      {!ready && <p role="status" className="py-5 text-center text-xs text-slate-500">Memuat halaman PDF…</p>}
      <canvas ref={canvas} aria-label={`${name}, halaman ${page}`} role="img" className={`mx-auto h-auto max-h-[600px] max-w-full ${ready ? "" : "hidden"}`} />
      {doc && doc.numPages > 1 && <div className="flex items-center justify-between gap-1 text-xs">
        <button type="button" disabled={page <= 1} className="text-primary-700 disabled:opacity-40" onClick={() => setPage((n) => n - 1)}>Sebelumnya</button>
        <span>{page} / {doc.numPages}</span>
        <button type="button" disabled={page >= doc.numPages} className="text-primary-700 disabled:opacity-40" onClick={() => setPage((n) => n + 1)}>Berikutnya</button>
      </div>}
    </div>
  );
}
