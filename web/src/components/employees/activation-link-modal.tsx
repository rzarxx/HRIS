"use client";

import { Check, Copy } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/modal";
import { dateTime } from "@/lib/format";
import type { Activation } from "@/lib/types";

/** Tautan hanya ada di layar ini: server menyimpan hash-nya saja. */
export function ActivationLinkModal({ name, activation, onClose }: { name: string; activation: Activation; onClose: () => void }) {
  const [copied, setCopied] = useState(false);
  const url = `${window.location.origin}/aktivasi/?token=${encodeURIComponent(activation.token)}`;

  async function copy() {
    await navigator.clipboard.writeText(url);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <Modal open title="Tautan aktivasi akun" size="sm" onClose={onClose} footer={<Button onClick={onClose}>Selesai</Button>}>
      <p className="text-sm text-slate-600">
        Teruskan tautan ini kepada <strong className="text-slate-800">{name}</strong> untuk menetapkan kata sandi. Berlaku sampai {dateTime(activation.expires_at)}, sekali pakai, dan tidak akan ditampilkan lagi.
      </p>
      <div className="mt-3 rounded-md bg-slate-50 p-3 font-mono text-xs break-all text-slate-700">{url}</div>
      <Button variant="secondary" size="sm" className="mt-3" onClick={copy}>
        {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
        {copied ? "Tersalin" : "Salin tautan"}
      </Button>
    </Modal>
  );
}
