"use client";

import { Check, ShieldCheck, X } from "lucide-react";
import { useState } from "react";
import { Button, IconButton } from "@/components/ui/button";
import { Field, Select, Textarea } from "@/components/ui/form";
import { Badge, Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog, Modal } from "@/components/ui/modal";
import { Pagination, Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { dateTime } from "@/lib/format";
import { useAdminDevices, useMe, useSave } from "@/lib/hooks";
import type { AdminDevice, DeviceIntegrityTier, DeviceStatus, DeviceTierInput } from "@/lib/types";

const PER_PAGE = 20;

const STATUS: Record<DeviceStatus, { label: string; tone: "amber" | "green" | "gray" }> = {
  pending: { label: "Menunggu", tone: "amber" },
  approved: { label: "Aktif", tone: "green" },
  revoked: { label: "Dicabut", tone: "gray" },
};

const TIER: Record<DeviceIntegrityTier, { label: string; tone: "green" | "accent" | "amber" | "gray" }> = {
  play: { label: "Play Integrity", tone: "green" },
  attestation: { label: "Key Attestation", tone: "accent" },
  manual: { label: "Manual (Review)", tone: "amber" },
  kiosk: { label: "Kiosk Kantor", tone: "gray" },
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
  const [tierDevice, setTierDevice] = useState<AdminDevice | null>(null);
  const [tierValue, setTierValue] = useState<DeviceIntegrityTier>("play");
  const [manualDays, setManualDays] = useState(30);
  const [tierReason, setTierReason] = useState("");
  const me = useMe();
  // Perangkat super admin hanya bisa dicabut/diubah super admin; server tetap menegakkan aturan ini (D34).
  const canManage = (d: AdminDevice) => d.status !== "revoked" && (!d.owner_is_superadmin || me.data?.role === "superadmin");

  const approve = useSave((id: string) => api.post(`/admin/devices/${id}/approve`), {
    success: "Perangkat disetujui.",
    onDone: () => setApproving(null),
  });
  const revoke = useSave((id: string) => api.post(`/admin/devices/${id}/revoke`), {
    success: "Perangkat dicabut.",
    onDone: () => setRevoking(null),
  });
  const updateTier = useSave(
    ({ id, body }: { id: string; body: DeviceTierInput }) => api.post(`/admin/devices/${id}/tier`, body),
    {
      success: "Jalur verifikasi perangkat diperbarui.",
      onDone: () => {
        setTierDevice(null);
        setTierReason("");
      },
    },
  );

  function openTierModal(d: AdminDevice) {
    setTierDevice(d);
    setTierValue(d.integrity_tier ?? "play");
    setManualDays(30);
    setTierReason("");
  }

  function submitTier() {
    if (!tierDevice) return;
    const until =
      tierValue === "manual" ? new Date(Date.now() + manualDays * 24 * 60 * 60 * 1000).toISOString() : null;
    updateTier.mutate({
      id: tierDevice.id,
      body: {
        integrity_tier: tierValue,
        manual_approved_until: until,
        reason: tierReason.trim(),
      },
    });
  }

  const isSelfDevice = tierDevice !== null && tierDevice.employee_id === me.data?.employee_id;

  return (
    <>
      <PageHeader title="Perangkat" description="Setiap akun hanya boleh presensi dari satu perangkat yang disetujui." />
      <Card bodyClassName="">
        <div className="border-b border-slate-100 p-4">
          <Select
            value={status}
            onChange={(e) => {
              setStatus(e.target.value);
              setPage(1);
            }}
            aria-label="Filter status"
            className="w-full sm:w-auto sm:min-w-48"
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
            <Table actions="always">
              <thead>
                <tr>
                  <Th>Pejuang</Th>
                  <Th>Perangkat</Th>
                  <Th>Verifikasi</Th>
                  <Th>Didaftarkan</Th>
                  <Th>Status</Th>
                  <Th align="right">
                    <span className="sr-only">Aksi</span>
                  </Th>
                </tr>
              </thead>
              <tbody>
                {list.data.data.map((d) => {
                  const tierInfo = TIER[d.integrity_tier ?? "play"] ?? TIER.play;
                  return (
                    <tr key={d.id} className="hover:bg-slate-50">
                      <Td>
                        <p className="font-medium text-slate-900">{d.employee_name}</p>
                        <p className="text-xs text-slate-500">
                          {d.nip} · {d.subdivision_name}
                        </p>
                      </Td>
                      <Td>{deviceName(d)}</Td>
                      <Td>
                        <Badge tone={tierInfo.tone}>{tierInfo.label}</Badge>
                        <p className="mt-1 text-xs text-slate-500">
                          {d.key_bound ? "Kunci instalasi terikat" : "Kunci belum terikat"}
                        </p>
                        {d.integrity_tier === "manual" && d.manual_approved_until && (
                          <p className="mt-0.5 text-xs text-amber-700">Berlaku s.d. {dateTime(d.manual_approved_until)}</p>
                        )}
                      </Td>
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
                        {canManage(d) && (
                          <IconButton label={`Ubah jalur verifikasi perangkat ${d.employee_name}`} onClick={() => openTierModal(d)} className="hover:text-primary-600">
                            <ShieldCheck className="size-4" />
                          </IconButton>
                        )}
                        {canManage(d) && (
                          <IconButton label={`Cabut perangkat ${d.employee_name}`} onClick={() => setRevoking(d)} className="hover:text-red-600">
                            <X className="size-4" />
                          </IconButton>
                        )}
                      </Td>
                    </tr>
                  );
                })}
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
            <strong className="text-slate-800">{approving && deviceName(approving)}</strong> akan menjadi perangkat presensi {approving?.employee_name}. Pastikan dulu permintaan ini benar dari yang bersangkutan.
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
      <Modal
        open={tierDevice !== null}
        title="Ubah jalur verifikasi perangkat"
        onClose={() => setTierDevice(null)}
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => setTierDevice(null)}>
              Batal
            </Button>
            <Button
              loading={updateTier.isPending}
              disabled={tierReason.trim().length < 3}
              onClick={submitTier}
            >
              Simpan
            </Button>
          </>
        }
      >
        {tierDevice && (
          <div className="space-y-4">
            <div className="rounded-md bg-slate-50 p-3 text-xs text-slate-600">
              <p className="font-medium text-slate-900">{tierDevice.employee_name}</p>
              <p>{deviceName(tierDevice)}</p>
            </div>
            <Field
              label="Jalur verifikasi"
              htmlFor="device-tier"
              hint={
                isSelfDevice
                  ? "Anda tidak dapat menurunkan perangkat sendiri ke jalur Manual atau Kiosk."
                  : "Gunakan jalur Manual hanya untuk HP sah tanpa GMS yang belum mendukung Key Attestation."
              }
            >
              <Select
                id="device-tier"
                value={tierValue}
                onChange={(e) => setTierValue(e.target.value as DeviceIntegrityTier)}
              >
                <option value="play">Play Integrity (Utama)</option>
                <option value="attestation">Android Key Attestation</option>
                <option value="manual" disabled={isSelfDevice}>
                  Manual / Tanpa GMS (Masuk antrean tinjau)
                </option>
                <option value="kiosk" disabled={isSelfDevice}>
                  Kiosk Kantor (Khusus di dalam geofence kantor)
                </option>
              </Select>
            </Field>
            {tierValue === "manual" && (
              <Field
                label="Masa berlaku persetujuan manual"
                htmlFor="manual-days"
                hint="Maksimal 90 hari; presensi dari perangkat ini akan selalu ditandai untuk ditinjau SDM."
              >
                <Select
                  id="manual-days"
                  value={String(manualDays)}
                  onChange={(e) => setManualDays(Number(e.target.value))}
                >
                  <option value="7">7 hari</option>
                  <option value="14">14 hari</option>
                  <option value="30">30 hari</option>
                  <option value="60">60 hari</option>
                  <option value="90">90 hari (Maksimal)</option>
                </Select>
              </Field>
            )}
            <Field label="Alasan perubahan" htmlFor="tier-reason" required hint="Minimal 3 karakter, tercatat di log audit.">
              <Textarea
                id="tier-reason"
                rows={3}
                maxLength={250}
                value={tierReason}
                onChange={(e) => setTierReason(e.target.value)}
                placeholder="Contoh: HP Huawei resmi tanpa Google Play Services, sudah diverifikasi fisik oleh SDM"
              />
            </Field>
          </div>
        )}
      </Modal>
    </>
  );
}
