"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Textarea } from "@/components/ui/form";
import { Modal } from "@/components/ui/modal";
import { api } from "@/lib/api-client";
import { useSave } from "@/lib/hooks";
import type { EmployeeListItem } from "@/lib/types";

/** Menonaktifkan template wajah agar Pejuang mendaftar ulang dari aplikasi. Tercatat di audit. */
export function FaceResetModal({ employee, onClose }: { employee: EmployeeListItem; onClose: () => void }) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const reset = useSave((r: string) => api.post(`/admin/employees/${employee.id}/face/reset`, { reason: r }), {
    success: "Wajah direset. Pejuang perlu mendaftar ulang dari aplikasi.",
    onDone: onClose,
    onFieldErrors: (f) => setError(f.reason ?? ""),
  });

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const value = reason.trim();
    if (!value) {
      setError("Alasan wajib diisi.");
      return;
    }
    reset.mutate(value);
  }

  return (
    <Modal
      open
      title="Reset wajah?"
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" form="face-reset-form" variant="danger" loading={reset.isPending}>
            Reset wajah
          </Button>
        </>
      }
    >
      <form id="face-reset-form" onSubmit={submit} noValidate className="space-y-4">
        <p className="text-sm text-slate-700">
          Wajah <strong className="text-slate-900">{employee.full_name}</strong> tidak bisa dipakai presensi lagi sampai ia mendaftar ulang dari aplikasi, di lokasi kantor atau dengan izin jarak jauh.
        </p>
        <Field label="Alasan" htmlFor="face_reset_reason" error={error} required>
          <Textarea
            id="face_reset_reason"
            rows={2}
            maxLength={500}
            value={reason}
            onChange={(e) => {
              setReason(e.target.value);
              setError("");
            }}
            invalid={!!error}
            placeholder="Misalnya: wajah tidak dikenali setelah berganti kacamata"
          />
        </Field>
      </form>
    </Modal>
  );
}
