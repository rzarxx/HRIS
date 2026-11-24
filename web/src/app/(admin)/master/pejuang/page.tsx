"use client";

import { CalendarClock, CalendarRange, FileSpreadsheet, KeyRound, Lock, LockOpen, MapPin, Pencil, Plus, ScanFace, Search, ShieldCheck, ShieldOff, UserCog, UserX, Video } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { BulkArrangementModal } from "@/components/employees/bulk-arrangement-modal";
import { EmployeeArrangementsModal } from "@/components/employees/employee-arrangements-modal";
import { EmployeeForm } from "@/components/employees/employee-form";
import { EmployeeLocationsModal } from "@/components/employees/employee-locations-modal";
import { FaceResetModal } from "@/components/employees/face-reset-modal";
import { ImportModal } from "@/components/employees/import-modal";
import { InitialPasswordModal } from "@/components/employees/initial-password-modal";
import { RemoteFaceModal } from "@/components/employees/remote-face-modal";
import { Button, IconButton } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/form";
import { Badge, Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog } from "@/components/ui/modal";
import { Pagination, Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { rupiah } from "@/lib/format";
import { useBranches, useDivisions, useEmployees, useMe, useSave, useSubdivisions } from "@/lib/hooks";
import type { EmployeeListItem, InitialCredential, ItemResponse } from "@/lib/types";

const PER_PAGE = 10;

function useDebounced<T>(value: T, ms = 300) {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
}

function EmployeesView() {
  const params = useSearchParams();
  const branches = useBranches();
  const divisions = useDivisions();
  const subdivisions = useSubdivisions();
  const me = useMe();
  const permissions = me.data?.permissions ?? [];
  const canWrite = permissions.includes("employee:write");
  const canReset = permissions.includes("user:reset");
  const canFilterOrg = permissions.includes("org:write");
  const canResetFace = permissions.includes("face:reset");
  const isSuperadmin = me.data?.role === "superadmin";
  // Peran SDM dan A2F wajib per akun hanya diatur super admin (D60).
  const canManageAccounts = permissions.includes("account:manage");
  // Akun super admin tidak bisa diubah SDM; server tetap menegakkan aturan ini (D34).
  const isProtected = (e: EmployeeListItem) => !isSuperadmin && e.account?.role === "superadmin";
  // Server menolak aksi ini untuk akun sendiri, jadi tombolnya tidak perlu tampil.
  const isSelf = (e: EmployeeListItem) => e.id === me.data?.employee_id;
  const [assigning, setAssigning] = useState<EmployeeListItem | null>(null);
  const [arranging, setArranging] = useState<EmployeeListItem | null>(null);
  const [resettingFace, setResettingFace] = useState<EmployeeListItem | null>(null);
  const [remoteFace, setRemoteFace] = useState<EmployeeListItem | null>(null);

  const [q, setQ] = useState(params.get("q") ?? "");
  const [branchId, setBranchId] = useState("");
  const [divisionId, setDivisionId] = useState("");
  const [subdivisionId, setSubdivisionId] = useState(params.get("subdivision_id") ?? "");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const debouncedQ = useDebounced(q);

  // Pencarian dari header bisa berubah saat halaman sudah terbuka
  const urlQ = params.get("q");
  useEffect(() => {
    if (urlQ !== null) setQ(urlQ);
  }, [urlQ]);
  useEffect(() => setPage(1), [debouncedQ, branchId, divisionId, subdivisionId, status]);

  const list = useEmployees({
    q: debouncedQ,
    branch_id: branchId,
    division_id: divisionId,
    subdivision_id: subdivisionId,
    status,
    page,
    per_page: PER_PAGE,
  });

  const [editing, setEditing] = useState<EmployeeListItem | "new" | null>(null);
  const [deleting, setDeleting] = useState<EmployeeListItem | null>(null);
  const remove = useSave((id: string) => api.post(`/admin/employees/${id}/deactivate`), {
    success: "Pejuang dinonaktifkan.",
    onDone: () => setDeleting(null),
  });

  const [bulkOpen, setBulkOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [credential, setCredential] = useState<{ name: string; nip?: string; password: string; expiresAt: string } | null>(null);
  const [resetting, setResetting] = useState<EmployeeListItem | null>(null);
  const resetPassword = useSave(
    (e: EmployeeListItem) => api.post<ItemResponse<InitialCredential>>(`/admin/users/${e.account!.user_id}/reset-password`),
    {
      success: "Kata sandi dikembalikan ke kata sandi awal. Sesi Pejuang ini dicabut.",
      onDone: (res) => {
        setCredential({ name: resetting?.full_name ?? "", nip: resetting?.nip, password: res.data.initial_password, expiresAt: res.data.expires_at });
        setResetting(null);
      },
    },
  );
  const setLock = useSave(
    ({ userId, lock }: { userId: string; lock: boolean }) => api.post(`/admin/users/${userId}/${lock ? "lock" : "unlock"}`),
    { success: "Status akun diperbarui." },
  );
  const [roleChange, setRoleChange] = useState<{ employee: EmployeeListItem; role: "sdm" | "pejuang" } | null>(null);
  const setRole = useSave(
    ({ userId, role }: { userId: string; role: "sdm" | "pejuang" }) => api.put(`/admin/users/${userId}/role`, { role }),
    { success: "Peran diperbarui. Sesi orang itu dicabut agar peran baru langsung berlaku.", onDone: () => setRoleChange(null) },
  );
  const [mfaChange, setMfaChange] = useState<{ employee: EmployeeListItem; required: boolean } | null>(null);
  const setMfaPolicy = useSave(
    ({ userId, required }: { userId: string; required: boolean }) => api.put(`/admin/users/${userId}/mfa-policy`, { required }),
    { success: "Pengaturan A2F diperbarui.", onDone: () => setMfaChange(null) },
  );

  const divOptions = (divisions.data ?? []).filter((d) => !branchId || d.branch_id === branchId);
  const subOptions = (subdivisions.data ?? []).filter((s) => (!divisionId || s.division_id === divisionId) && (!branchId || s.branch_id === branchId));

  return (
    <>
      <PageHeader
        title="Pejuang"
        description="Data Pejuang aktif dan nonaktif."
        actions={
          canWrite && (
            <>
              <Button variant="secondary" onClick={() => setImportOpen(true)}>
                <FileSpreadsheet className="size-4" /> Impor xlsx
              </Button>
              <Button variant="secondary" onClick={() => setBulkOpen(true)}>
                <CalendarRange className="size-4" /> Izin massal
              </Button>
              <Button onClick={() => setEditing("new")}>
                <Plus className="size-4" /> Tambah Pejuang
              </Button>
            </>
          )
        }
      />
      <Card bodyClassName="">
        <div className="flex flex-wrap gap-3 border-b border-slate-100 p-4">
          <div className="relative w-full max-w-xs">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden />
            <Input
              type="search"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Cari nama, NIPY, email, WA, jabatan..."
              aria-label="Cari Pejuang"
              className="pl-9"
            />
          </div>
          {canFilterOrg && (
            <>
              {(branches.data?.length ?? 0) > 1 && (
                <Select
                  value={branchId}
                  onChange={(e) => {
                    setBranchId(e.target.value);
                    setDivisionId("");
                    setSubdivisionId("");
                  }}
                  aria-label="Filter cabang"
                  className="w-full sm:w-auto sm:min-w-40"
                >
                  <option value="">Semua cabang</option>
                  {branches.data?.map((b) => (
                    <option key={b.id} value={b.id}>
                      {b.name}
                    </option>
                  ))}
                </Select>
              )}
              <Select
                value={divisionId}
                onChange={(e) => {
                  setDivisionId(e.target.value);
                  setSubdivisionId("");
                }}
                aria-label="Filter divisi"
                className="w-full sm:w-auto sm:min-w-44"
              >
                <option value="">Semua divisi</option>
                {divOptions.map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.name}
                    {!branchId && (branches.data?.length ?? 0) > 1 ? ` · ${d.branch_name}` : ""}
                  </option>
                ))}
              </Select>
              <Select value={subdivisionId} onChange={(e) => setSubdivisionId(e.target.value)} aria-label="Filter subdiv" className="w-full sm:w-auto sm:min-w-44">
                <option value="">Semua subdiv</option>
                {subOptions.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                    {!divisionId && (branches.data?.length ?? 0) > 1 ? ` · ${s.branch_name}` : ""}
                  </option>
                ))}
              </Select>
            </>
          )}
          <Select value={status} onChange={(e) => setStatus(e.target.value)} aria-label="Filter status" className="w-full sm:w-auto">
            <option value="">Semua status</option>
            <option value="active">Aktif</option>
            <option value="inactive">Nonaktif</option>
          </Select>
        </div>

        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : list.data.data.length === 0 ? (
          <EmptyState title="Pejuang tidak ditemukan" description="Ubah kata kunci atau filter pencarian." />
        ) : (
          <>
            <Table actions="sm">
              <thead>
                <tr>
                  <Th>NIPY</Th>
                  <Th>Nama</Th>
                  <Th>Jabatan / Shift</Th>
                  <Th>Subdiv / Divisi</Th>
                  {!isSuperadmin && <Th align="right">Ubar pokok</Th>}
                  <Th>Status</Th>
                  <Th align="right">
                    <span className="sr-only">Aksi</span>
                  </Th>
                </tr>
              </thead>
              <tbody className={list.isPlaceholderData ? "opacity-60" : ""}>
                {list.data.data.map((e) => (
                  <tr key={e.id} className="hover:bg-slate-50">
                    <Td className="font-mono text-xs whitespace-nowrap">{e.nip}</Td>
                    <Td>
                      <p className="font-medium text-slate-900">{e.full_name}</p>
                      <p className="text-xs text-slate-500">{e.email ?? e.phone ?? "-"}</p>
                    </Td>
                    <Td>
                      <p>{e.position ?? "-"}</p>
                      <p className="text-xs text-slate-500">{e.default_shift_name ?? "Shift belum ditentukan"}</p>
                    </Td>
                    <Td>
                      <p>{e.subdivision_name}</p>
                      <p className="text-xs text-slate-500">
                        {e.division_name}
                        {(branches.data?.length ?? 0) > 1 && ` · ${e.branch_name}`}
                      </p>
                    </Td>
                    {!isSuperadmin && (
                      <Td align="right" className="tabular-nums whitespace-nowrap">
                        {e.base_salary ? rupiah(e.base_salary) : "-"}
                      </Td>
                    )}
                    <Td>
                      <div className="flex flex-col items-start gap-1">
                        {e.status === "active" ? <Badge tone="green">Aktif</Badge> : <Badge>Nonaktif</Badge>}
                        {e.account?.is_locked && <Badge tone="red">Terkunci</Badge>}
                        {e.account?.role === "sdm" && <Badge tone="accent">SDM</Badge>}
                        {e.account?.role === "superadmin" && <Badge tone="accent">Super admin</Badge>}
                        {e.account?.role === "sdm" && !e.account.mfa_required && <Badge tone="amber">A2F opsional</Badge>}
                        {e.status === "active" && e.account?.is_active && !e.face_enrolled && <Badge tone="amber">Wajah belum terdaftar</Badge>}
                      </div>
                    </Td>
                    <Td align="right" className="whitespace-nowrap">
                      {canWrite && !isProtected(e) && (
                        <IconButton label={`Ubah ${e.full_name}`} onClick={() => setEditing(e)}>
                          <Pencil className="size-4" />
                        </IconButton>
                      )}
                      {canWrite && !isProtected(e) && e.status === "active" && (
                        <IconButton label={`Lokasi presensi ${e.full_name}`} onClick={() => setAssigning(e)}>
                          <MapPin className="size-4" />
                        </IconButton>
                      )}
                      {canWrite && !isProtected(e) && e.status === "active" && (
                        <IconButton label={`Izin kerja ${e.full_name}`} onClick={() => setArranging(e)}>
                          <CalendarClock className="size-4" />
                        </IconButton>
                      )}
                      {canResetFace && e.face_enrolled && !isProtected(e) && !isSelf(e) && (
                        <IconButton label={`Reset wajah ${e.full_name}`} onClick={() => setResettingFace(e)}>
                          <ScanFace className="size-4" />
                        </IconButton>
                      )}
                      {canResetFace && e.status === "active" && !isProtected(e) && !isSelf(e) && (
                        <IconButton label={`Izin daftar wajah jarak jauh ${e.full_name}`} onClick={() => setRemoteFace(e)}>
                          <Video className="size-4" />
                        </IconButton>
                      )}
                      {e.status === "active" && e.account && !isProtected(e) && !isSelf(e) && (
                        <>
                          {canReset && !(isSuperadmin && e.account.role === "sdm") && (
                            <IconButton label={`Reset kata sandi ${e.full_name}`} onClick={() => setResetting(e)}>
                              <KeyRound className="size-4" />
                            </IconButton>
                          )}
                          {e.account.is_locked ? (
                            <IconButton label={`Buka kunci akun ${e.full_name}`} onClick={() => setLock.mutate({ userId: e.account!.user_id, lock: false })}>
                              <LockOpen className="size-4" />
                            </IconButton>
                          ) : (
                            <IconButton label={`Kunci akun ${e.full_name}`} onClick={() => setLock.mutate({ userId: e.account!.user_id, lock: true })}>
                              <Lock className="size-4" />
                            </IconButton>
                          )}
                        </>
                      )}
                      {canManageAccounts && e.account && !isSelf(e) && e.status === "active" && (e.account.role === "pejuang" || e.account.role === "sdm") && (
                        <IconButton
                          label={e.account.role === "sdm" ? `Kembalikan ${e.full_name} ke Pejuang` : `Jadikan ${e.full_name} SDM`}
                          onClick={() => setRoleChange({ employee: e, role: e.account!.role === "sdm" ? "pejuang" : "sdm" })}
                        >
                          <UserCog className="size-4" />
                        </IconButton>
                      )}
                      {canManageAccounts && e.account?.role === "sdm" && !isSelf(e) && e.status === "active" && (
                        <IconButton
                          label={e.account.mfa_required ? `Jadikan A2F opsional untuk ${e.full_name}` : `Wajibkan A2F untuk ${e.full_name}`}
                          onClick={() => setMfaChange({ employee: e, required: !e.account!.mfa_required })}
                        >
                          {e.account.mfa_required ? <ShieldOff className="size-4" /> : <ShieldCheck className="size-4" />}
                        </IconButton>
                      )}
                      {canWrite && !isProtected(e) && !isSelf(e) && e.status === "active" && (
                        <IconButton label={`Nonaktifkan ${e.full_name}`} onClick={() => setDeleting(e)} className="hover:text-red-600">
                          <UserX className="size-4" />
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

      {editing !== null && (
        <EmployeeForm
          employee={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
          onCreated={(c) => setCredential({ name: c.full_name, nip: c.nip, password: c.initial_password, expiresAt: c.expires_at })}
        />
      )}

      {assigning && <EmployeeLocationsModal employee={assigning} onClose={() => setAssigning(null)} />}

      {arranging && <EmployeeArrangementsModal employee={arranging} onClose={() => setArranging(null)} />}

      {resettingFace && <FaceResetModal employee={resettingFace} onClose={() => setResettingFace(null)} />}
      {remoteFace && <RemoteFaceModal employee={remoteFace} onClose={() => setRemoteFace(null)} />}

      {bulkOpen && <BulkArrangementModal onClose={() => setBulkOpen(false)} />}
      {importOpen && <ImportModal onClose={() => setImportOpen(false)} />}

      {roleChange && (
        <ConfirmDialog
          open
          title={roleChange.role === "sdm" ? "Jadikan SDM?" : "Kembalikan ke Pejuang?"}
          message={
            roleChange.role === "sdm" ? (
              <>
                <span className="font-medium">{roleChange.employee.full_name}</span> mendapat akses SDM: data seluruh Pejuang, ubar, payroll, dan
                persetujuan. Sesinya dicabut, dan saat masuk lagi wajib mengaktifkan A2F kecuali Anda menjadikannya opsional.
              </>
            ) : (
              <>
                Akses SDM <span className="font-medium">{roleChange.employee.full_name}</span> dicabut dan sesinya dikeluarkan.
              </>
            )
          }
          confirmLabel={roleChange.role === "sdm" ? "Jadikan SDM" : "Kembalikan"}
          loading={setRole.isPending}
          onConfirm={() => setRole.mutate({ userId: roleChange.employee.account!.user_id, role: roleChange.role })}
          onClose={() => setRoleChange(null)}
        />
      )}

      {mfaChange && (
        <ConfirmDialog
          open
          title={mfaChange.required ? "Wajibkan A2F?" : "Jadikan A2F opsional?"}
          message={
            mfaChange.required ? (
              <>
                <span className="font-medium">{mfaChange.employee.full_name}</span> wajib mengaktifkan A2F sebelum memakai fitur lain.
              </>
            ) : (
              <>
                <span className="font-medium">{mfaChange.employee.full_name}</span> bisa masuk tanpa A2F. Akun SDM tanpa A2F lebih mudah diambil alih bila
                kata sandinya bocor. A2F yang sudah aktif tetap diminta saat masuk.
              </>
            )
          }
          confirmLabel={mfaChange.required ? "Wajibkan" : "Jadikan opsional"}
          loading={setMfaPolicy.isPending}
          onConfirm={() => setMfaPolicy.mutate({ userId: mfaChange.employee.account!.user_id, required: mfaChange.required })}
          onClose={() => setMfaChange(null)}
        />
      )}

      {credential && <InitialPasswordModal name={credential.name} nip={credential.nip} password={credential.password} expiresAt={credential.expiresAt} onClose={() => setCredential(null)} />}

      <ConfirmDialog
        open={resetting !== null}
        title="Reset ke kata sandi awal?"
        confirmLabel="Reset"
        message={
          <>
            Kata sandi lama <strong className="text-slate-800">{resetting?.full_name}</strong> tidak berlaku lagi dan seluruh sesinya dicabut. Akun kembali ke
            kata sandi awal yang wajib diganti saat login berikutnya.
          </>
        }
        loading={resetPassword.isPending}
        onConfirm={() => resetting && resetPassword.mutate(resetting)}
        onClose={() => setResetting(null)}
      />

      <ConfirmDialog
        open={deleting !== null}
        title="Nonaktifkan Pejuang?"
        confirmLabel="Nonaktifkan"
        message={
          <>
            <strong className="text-slate-800">{deleting?.full_name}</strong> ({deleting?.nip}) akan dinonaktifkan, akunnya tidak bisa masuk lagi, dan tidak lagi dihitung di grafik. Riwayat presensi dan ubar tetap
            tersimpan.
          </>
        }
        loading={remove.isPending}
        onConfirm={() => deleting && remove.mutate(deleting.id)}
        onClose={() => setDeleting(null)}
      />
    </>
  );
}

export default function EmployeesPage() {
  return (
    <Suspense fallback={<Loading />}>
      <EmployeesView />
    </Suspense>
  );
}
