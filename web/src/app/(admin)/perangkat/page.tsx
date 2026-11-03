"use client";

import { Check, X } from "lucide-react";
import { useState } from "react";
import { IconButton } from "@/components/ui/button";
import { Select } from "@/components/ui/form";
import { Badge, Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog } from "@/components/ui/modal";
import { Pagination, Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { dateTime } from "@/lib/format";
import { useAdminDevices, useSave } from "@/lib/hooks";
import type { AdminDevice, DeviceStatus } from "@/lib/types";

const PER_PAGE = 20;

const STATUS: Record<DeviceStatus, { label: string; tone: "amber" | "green" | "gray" }> = {
  pending: { label: "Menunggu", tone: "amber" },
  approved: { label: "Aktif", tone: "green" },
  revoked: { label: "Dicabut", tone: "gray" },
};

const REVOKE_REASON: Record<string, string> = {
  admin_revoked: "Dicabut admin",
  replaced: "Diganti perangkat baru",
  offboarded: "Pejuang nonaktif",
};

function deviceName(d: AdminDevice) {
  return [d.model, d.os_version].filter(Boolean).join(" · ") || "Perangkat tanpa nama";
}

export default function DevicesPage() {
  const [status, setStatus] = useState("pending");
  const [page, setPage] = useState(1);
  const list = useAdminDevices({ status, page, per_page: PER_PAGE });
  const [approving, setApproving] = useState<AdminDevice | null>(null);
  const [revoking, setRevoking] = useState<AdminDevice | null>(null);

  const approve = useSave((id: string) => api.post(`/admin/devices/${id}/approve`), {
    success: "Perangkat disetujui.",
    onDone: () => setApproving(null),
  });
  const revoke = useSave((id: string) => api.post(`/admin/devices/${id}/revoke`), {
    success: "Perangkat dicabut.",
    onDone: () => setRevoking(null),
  });

  return (
    <>
      <PageHeader title="Perangkat" description="Setiap akun hanya boleh absen dari satu perangkat yang disetujui." />
      <Card bodyClassName="">
        <div className="border-b border-slate-100 p-4">
          <Select
            value={status}
            onChange={(e) => {
              setStatus(e.target.value);
              setPage(1);
            }}
            aria-label="Filter status"
            className="w-auto min-w-48"
          >
            <option value="pending">Menunggu persetujuan</option>
            <option value="approved">Aktif</option>
            <option value="revoked">Dicabut</option>
            <option value="">Semua status</option>
          </Select>
        </div>
        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : list.data.data.length === 0 ? (
          <EmptyState title="Tidak ada perangkat" description="Perangkat baru dari aplikasi Android akan muncul di sini." />
        ) : (
          <>
            <Table>
              <thead>
                <tr>
                  <Th>Pejuang</Th>
                  <Th>Perangkat</Th>
                  <Th>Didaftarkan</Th>
                  <Th>Status</Th>
                  <Th align="right">
                    <span className="sr-only">Aksi</span>
                  </Th>
                </tr>
              </thead>
              <tbody>
                {list.data.data.map((d) => (
                  <tr key={d.id} className="hover:bg-slate-50">
                    <Td>
                      <p className="font-medium text-slate-900">{d.employee_name}</p>
                      <p className="text-xs text-slate-500">
                        {d.nip} · {d.subdivision_name}
                      </p>
                    </Td>
                    <Td>{deviceName(d)}</Td>
                    <Td className="whitespace-nowrap">{dateTime(d.created_at)}</Td>
                    <Td>
                      <Badge tone={STATUS[d.status].tone}>{STATUS[d.status].label}</Badge>
                      {d.status === "approved" && <p className="mt-1 text-xs text-slate-500">{d.approved_by_name ? `Oleh ${d.approved_by_name}` : "Perangkat pertama"}</p>}
                      {d.revoked_reason && <p className="mt-1 text-xs text-slate-500">{REVOKE_REASON[d.revoked_reason] ?? d.revoked_reason}</p>}
                    </Td>
                    <Td align="right" className="whitespace-nowrap">
                      {d.status === "pending" && (
                        <IconButton label={`Setujui perangkat ${d.employee_name}`} onClick={() => setApproving(d)} className="hover:text-emerald-600">
                          <Check className="size-4" />
                        </IconButton>
                      )}
                      {d.status !== "revoked" && (
                        <IconButton label={`Cabut perangkat ${d.employee_name}`} onClick={() => setRevoking(d)} className="hover:text-red-600">
                          <X className="size-4" />
                        </IconButton>
                      )}
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
            <Pagination page={page} perPage={PER_PAGE} total={list.data.meta.total} onPage={setPage} />
          </>
        )}
      </Card>

      <ConfirmDialog
        open={approving !== null}
        title="Setujui perangkat?"
        confirmLabel="Setujui"
        message={
          <>
            <strong className="text-slate-800">{approving && deviceName(approving)}</strong> akan menjadi perangkat absen {approving?.employee_name}. Pastikan dulu permintaan ini benar dari yang bersangkutan.
            Perangkat lamanya otomatis dicabut.
          </>
        }
        loading={approve.isPending}
        onConfirm={() => approving && approve.mutate(approving.id)}
        onClose={() => setApproving(null)}
      />
      <ConfirmDialog
        open={revoking !== null}
        title="Cabut perangkat?"
        confirmLabel="Cabut"
        message={
          <>
            <strong className="text-slate-800">{revoking && deviceName(revoking)}</strong> milik {revoking?.employee_name} tidak bisa dipakai lagi dan sesinya dicabut. Perangkat yang dicabut tidak bisa disetujui kembali.
          </>
        }
        loading={revoke.isPending}
        onConfirm={() => revoking && revoke.mutate(revoking.id)}
        onClose={() => setRevoking(null)}
      />
    </>
  );
}
