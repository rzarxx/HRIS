"use client";

import { AlertCircle, Check, Copy } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/modal";
import { copyText } from "@/lib/clipboard";
import { dateTime } from "@/lib/format";
import type { Activation } from "@/lib/types";

const COPIED_MS = 2500;

/** Tautan hanya ada di layar ini: server menyimpan hash-nya saja. */
export function ActivationLinkModal({ name, activation, onClose }: { name: string; activation: Activation; onClose: () => void }) {
  const [status, setStatus] = useState<"idle" | "copied" | "failed">("idle");
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const link = useRef<HTMLDivElement>(null);
  const url = `${window.location.origin}/aktivasi/?token=${encodeURIComponent(activation.token)}`;

  useEffect(() => () => clearTimeout(timer.current), []);

  async function copy() {
    clearTimeout(timer.current);
    if (await copyText(url)) {
      setStatus("copied");
      timer.current = setTimeout(() => setStatus("idle"), COPIED_MS);
      return;
    }
    setStatus("failed");
    // Tautan tidak bisa diterbitkan ulang tanpa membatalkan yang ini, jadi siapkan untuk disalin manual.
    if (link.current) window.getSelection()?.selectAllChildren(link.current);
  }

  return (
    <Modal open title="Tautan aktivasi akun" size="sm" onClose={onClose} footer={<Button onClick={onClose}>Selesai</Button>}>
      <p className="text-sm text-slate-600">
        Teruskan tautan ini kepada <strong className="text-slate-800">{name}</strong> untuk menetapkan kata sandi. Berlaku sampai {dateTime(activation.expires_at)}, sekali pakai, dan tidak akan ditampilkan lagi.
      </p>
      <div ref={link} className="mt-3 rounded-md bg-slate-50 p-3 font-mono text-xs break-all text-slate-700 select-all">
        {url}
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-3">
        <Button
          variant={status === "copied" ? "success" : "secondary"}
          size="sm"
          onClick={copy}
        >
          {status === "copied" ? <Check className="size-4" aria-hidden /> : <Copy className="size-4" aria-hidden />}
          {status === "copied" ? "Tersalin" : "Salin tautan"}
        </Button>
        <p role="status" className="text-xs">
          {status === "copied" && <span className="text-primary-700">Tautan sudah tersalin, tinggal ditempel.</span>}
          {status === "failed" && (
            <span className="inline-flex items-start gap-1 text-red-700">
              <AlertCircle className="mt-px size-3.5 shrink-0" aria-hidden />
              Browser menolak menyalin. Tautan sudah dipilih, tekan Ctrl+C.
            </span>
          )}
        </p>
      </div>
    </Modal>
  );
}
