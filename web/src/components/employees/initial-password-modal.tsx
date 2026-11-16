"use client";

import { AlertCircle, Check, Copy } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/modal";
import { copyText } from "@/lib/clipboard";

const COPIED_MS = 2500;

/** Kata sandi awal hanya tampil di layar ini; server menyimpan hash-nya saja (D53). */
export function InitialPasswordModal({ name, nip, password, onClose }: { name: string; nip?: string; password: string; onClose: () => void }) {
  const [status, setStatus] = useState<"idle" | "copied" | "failed">("idle");
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const box = useRef<HTMLDivElement>(null);

  useEffect(() => () => clearTimeout(timer.current), []);

  async function copy() {
    clearTimeout(timer.current);
    if (await copyText(password)) {
      setStatus("copied");
      timer.current = setTimeout(() => setStatus("idle"), COPIED_MS);
      return;
    }
    setStatus("failed");
    if (box.current) window.getSelection()?.selectAllChildren(box.current);
  }

  return (
    <Modal open title="Kata sandi awal" size="sm" onClose={onClose} footer={<Button onClick={onClose}>Selesai</Button>}>
      <p className="text-sm text-slate-600">
        Sampaikan kata sandi ini kepada <strong className="text-slate-800">{name}</strong>. Pejuang masuk dengan NIPY
        {nip ? <> (<span className="font-mono">{nip}</span>)</> : null}, email, atau nomor WA, lalu wajib mengganti kata sandi saat login pertama.
      </p>
      <div ref={box} className="mt-3 rounded-md bg-slate-50 p-3 font-mono text-base break-all text-slate-800 select-all">
        {password}
      </div>
      <p className="mt-2 text-xs text-slate-500">Kata sandi ini tidak akan ditampilkan lagi. Bila terlupa, gunakan Reset kata sandi.</p>
      <div className="mt-3 flex flex-wrap items-center gap-3">
        <Button variant={status === "copied" ? "success" : "secondary"} size="sm" onClick={copy}>
          {status === "copied" ? <Check className="size-4" aria-hidden /> : <Copy className="size-4" aria-hidden />}
          {status === "copied" ? "Tersalin" : "Salin kata sandi"}
        </Button>
        <p role="status" className="text-xs">
          {status === "copied" && <span className="text-primary-700">Kata sandi sudah tersalin.</span>}
          {status === "failed" && (
            <span className="inline-flex items-start gap-1 text-red-700">
              <AlertCircle className="mt-px size-3.5 shrink-0" aria-hidden />
              Browser menolak menyalin. Teks sudah dipilih, tekan Ctrl+C.
            </span>
          )}
        </p>
      </div>
    </Modal>
  );
}
