"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Textarea } from "@/components/ui/form";
import { ErrorState, Loading } from "@/components/ui/layout";
import { Modal } from "@/components/ui/modal";
import { api } from "@/lib/api-client";
import { dateTime } from "@/lib/format";
import { useSave } from "@/lib/hooks";
import type { EmployeeListItem, ItemResponse } from "@/lib/types";
import type { components } from "@/api/schema";

type Permission = components["schemas"]["RemoteFacePermission"];

export function RemoteFaceModal({ employee, onClose }: { employee: EmployeeListItem; onClose: () => void }) {
  const path = `/admin/employees/${employee.id}/face/remote-permission`;
  const status = useQuery({ queryKey: ["remote-face", employee.id], queryFn: () => api.get<ItemResponse<Permission | null>>(path).then((v) => v.data) });
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const grant = useSave((value: string) => api.put(path, { reason: value }), { success: "Izin pendaftaran wajah diberikan.", onFieldErrors: (f) => setError(f.reason ?? ""), onDone: () => setReason("") });
  const revoke = useSave<void>(() => api.del(path), { success: "Izin pendaftaran wajah dicabut." });
  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!reason.trim()) { setError("Alasan wajib diisi."); return; }
    grant.mutate(reason.trim());
  }
  return <Modal open title={`Izin daftar wajah ${employee.full_name}`} onClose={onClose} footer={<Button variant="secondary" onClick={onClose}>Tutup</Button>}>
    {status.isPending ? <Loading /> : status.isError ? <ErrorState message="Izin tidak dapat dimuat." onRetry={() => status.refetch()} /> : <div className="space-y-4">
      <p className="text-sm text-slate-700">Izin berlaku 24 jam untuk satu pendaftaran wajah di luar kantor. Perangkat dan pemeriksaan keamanan tetap diwajibkan.</p>
      {status.data && <div className="rounded-md bg-amber-50 p-3 text-sm text-amber-900">
        <p>Izin aktif sampai {dateTime(status.data.expires_at)}.</p><p>Alasan: {status.data.reason}</p>
        <Button type="button" variant="danger" loading={revoke.isPending} onClick={() => revoke.mutate()}>Cabut izin</Button>
      </div>}
      <form onSubmit={submit} className="space-y-3">
        <Field label="Alasan" htmlFor="remote-face-reason" error={error} required>
          <Textarea id="remote-face-reason" rows={2} maxLength={500} value={reason} onChange={(e) => { setReason(e.target.value); setError(""); }} invalid={!!error} />
        </Field>
        <Button type="submit" loading={grant.isPending}>{status.data ? "Perbarui izin 24 jam" : "Berikan izin"}</Button>
      </form>
    </div>}
  </Modal>;
}
