"use client";

import { CalendarClock, KeyRound, Lock, LockOpen, MapPin, Pencil, Plus, Search, UserX } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { ActivationLinkModal } from "@/components/employees/activation-link-modal";
import { EmployeeArrangementsModal } from "@/components/employees/employee-arrangements-modal";
import { EmployeeForm } from "@/components/employees/employee-form";
import { EmployeeLocationsModal } from "@/components/employees/employee-locations-modal";
import { Button, IconButton } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/form";
import { Badge, Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog } from "@/components/ui/modal";
import { Pagination, Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { rupiah } from "@/lib/format";
import { useDivisions, useEmployees, useMe, useSave, useSubdivisions } from "@/lib/hooks";
import type { Activation, EmployeeListItem, ItemResponse } from "@/lib/types";

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
  const divisions = useDivisions();
  const subdivisions = useSubdivisions();
  const me = useMe();
  const permissions = me.data?.permissions ?? [];
  const canWrite = permissions.includes("employee:write");
  const canReset = permissions.includes("user:reset");
  const canFilterOrg = permissions.includes("org:write");
  const isSuperadmin = me.data?.role === "superadmin";
  // Akun super admin tidak bisa diubah SDM; server tetap menegakkan aturan ini (D34).
  const isProtected = (e: EmployeeListItem) => !isSuperadmin && e.account?.role === "superadmin";
  // Server menolak aksi ini untuk akun sendiri, jadi tombolnya tidak perlu tampil.
  const isSelf = (e: EmployeeListItem) => e.id === me.data?.employee_id;
  const [assigning, setAssigning] = useState<EmployeeListItem | null>(null);
  const [arranging, setArranging] = useState<EmployeeListItem | null>(null);

  const [q, setQ] = useState(params.get("q") ?? "");
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
  useEffect(() => setPage(1), [debouncedQ, divisionId, subdivisionId, status]);

  const list = useEmployees({
    q: debouncedQ,
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

  const [activation, setActivation] = useState<{ name: string; activation: Activation } | null>(null);
  const [resetting, setResetting] = useState<EmployeeListItem | null>(null);
  const resetPassword = useSave(
    (e: EmployeeListItem) => api.post<ItemResponse<Activation>>(`/admin/users/${e.account!.user_id}/reset-password`),
    {
      success: "Tautan baru diterbitkan. Sesi Pejuang ini dicabut.",
      onDone: (res) => {
        setActivation({ name: resetting?.full_name ?? "", activation: res.data });
        setResetting(null);
      },
    },
  );
  const setLock = useSave(
    ({ userId, lock }: { userId: string; lock: boolean }) => api.post(`/admin/users/${userId}/${lock ? "lock" : "unlock"}`),
    { success: "Status akun diperbarui." },
  );

  const subOptions = (subdivisions.data ?? []).filter((s) => !divisionId || s.division_id === divisionId);

  return (
    <>
      <PageHeader
        title="Pejuang"
        description="Data Pejuang aktif dan nonaktif."
        actions={
          canWrite && (
            <Button onClick={() => setEditing("new")}>
              <Plus className="size-4" /> Tambah Pejuang
            </Button>
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
              placeholder="Cari nama, NIPY, email, jabatan..."
              aria-label="Cari Pejuang"
              className="pl-9"
            />
          </div>
          {canFilterOrg && (
            <>
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
                {divisions.data?.map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.name}
                  </option>
                ))}
              </Select>
              <Select value={subdivisionId} onChange={(e) => setSubdivisionId(e.target.value)} aria-label="Filter subdiv" className="w-full sm:w-auto sm:min-w-44">
                <option value="">Semua subdiv</option>
                {subOptions.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
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
                  {!isSuperadmin && <Th align="right">Gaji pokok</Th>}
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
                      <p className="text-xs text-slate-500">{e.email ?? "-"}</p>
                    </Td>
                    <Td>
                      <p>{e.position ?? "-"}</p>
                      <p className="text-xs text-slate-500">{e.default_shift_name ?? "Shift belum ditentukan"}</p>
                    </Td>
                    <Td>
                      <p>{e.subdivision_name}</p>
                      <p className="text-xs text-slate-500">{e.division_name}</p>
                    </Td>
                    {!isSuperadmin && (
                      <Td align="right" className="tabular-nums whitespace-nowrap">
                        {e.base_salary ? rupiah(e.base_salary) : "-"}
                      </Td>
                    )}
                    <Td>
                      <div className="flex flex-col items-start gap-1">
                        {e.status === "active" ? <Badge tone="green">Aktif</Badge> : <Badge>Nonaktif</Badge>}
                        {e.status === "active" && e.account && !e.account.is_active && <Badge tone="amber">Belum aktivasi</Badge>}
                        {e.account?.is_locked && <Badge tone="red">Terkunci</Badge>}
                      </div>
                    </Td>
                    <Td align="right" className="whitespace-nowrap">
                      {canWrite && !isProtected(e) && (
                        <IconButton label={`Ubah ${e.full_name}`} onClick={() => setEditing(e)}>
                          <Pencil className="size-4" />
                        </IconButton>
                      )}
                      {canWrite && !isProtected(e) && e.status === "active" && (
                        <IconButton label={`Lokasi absen ${e.full_name}`} onClick={() => setAssigning(e)}>
                          <MapPin className="size-4" />
                        </IconButton>
                      )}
                      {canWrite && !isProtected(e) && e.status === "active" && (
                        <IconButton label={`Izin kerja ${e.full_name}`} onClick={() => setArranging(e)}>
                          <CalendarClock className="size-4" />
                        </IconButton>
                      )}
                      {e.status === "active" && e.account && !isProtected(e) && !isSelf(e) && (
                        <>
                          {canReset && !(isSuperadmin && e.account.role === "sdm") && (
                            <IconButton label={`Terbitkan tautan sandi untuk ${e.full_name}`} onClick={() => setResetting(e)}>
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
          onCreated={(c) => setActivation({ name: c.full_name, activation: c.activation })}
        />
      )}

      {assigning && <EmployeeLocationsModal employee={assigning} onClose={() => setAssigning(null)} />}

      {arranging && <EmployeeArrangementsModal employee={arranging} onClose={() => setArranging(null)} />}

      {activation && <ActivationLinkModal name={activation.name} activation={activation.activation} onClose={() => setActivation(null)} />}

      <ConfirmDialog
        open={resetting !== null}
        title="Terbitkan tautan kata sandi baru?"
        confirmLabel="Terbitkan"
        message={
          <>
            Kata sandi lama <strong className="text-slate-800">{resetting?.full_name}</strong> tidak berlaku lagi dan seluruh sesinya dicabut. Anda mendapat
            tautan untuk diteruskan kepada yang bersangkutan.
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
            <strong className="text-slate-800">{deleting?.full_name}</strong> ({deleting?.nip}) akan dinonaktifkan, akunnya tidak bisa masuk lagi, dan tidak lagi dihitung di grafik. Riwayat presensi dan upah tetap
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
