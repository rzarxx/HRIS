"use client";

import { Pencil, Plus, Search, Trash2 } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { EmployeeForm } from "@/components/employees/employee-form";
import { Button, IconButton } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/form";
import { Badge, Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog } from "@/components/ui/modal";
import { Pagination, Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { rupiah } from "@/lib/format";
import { useDivisions, useEmployees, useSave, useSubdivisions } from "@/lib/hooks";
import type { EmployeeListItem } from "@/lib/types";

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
  const remove = useSave((id: string) => api.del(`/admin/employees/${id}`), {
    success: "Data Pejuang dihapus.",
    onDone: () => setDeleting(null),
  });

  const subOptions = (subdivisions.data ?? []).filter((s) => !divisionId || s.division_id === divisionId);

  return (
    <>
      <PageHeader
        title="Pejuang"
        description="Data karyawan aktif dan nonaktif."
        actions={
          <Button onClick={() => setEditing("new")}>
            <Plus className="size-4" /> Tambah Pejuang
          </Button>
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
              placeholder="Cari nama, NIP, email, jabatan..."
              aria-label="Cari Pejuang"
              className="pl-9"
            />
          </div>
          <Select
            value={divisionId}
            onChange={(e) => {
              setDivisionId(e.target.value);
              setSubdivisionId("");
            }}
            aria-label="Filter divisi"
            className="w-auto min-w-44"
          >
            <option value="">Semua divisi</option>
            {divisions.data?.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
              </option>
            ))}
          </Select>
          <Select value={subdivisionId} onChange={(e) => setSubdivisionId(e.target.value)} aria-label="Filter subdiv" className="w-auto min-w-44">
            <option value="">Semua subdiv</option>
            {subOptions.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </Select>
          <Select value={status} onChange={(e) => setStatus(e.target.value)} aria-label="Filter status" className="w-auto">
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
            <Table>
              <thead>
                <tr>
                  <Th>NIP</Th>
                  <Th>Nama</Th>
                  <Th>Jabatan</Th>
                  <Th>Subdiv / Divisi</Th>
                  <Th align="right">Gaji pokok</Th>
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
                    <Td>{e.position ?? "-"}</Td>
                    <Td>
                      <p>{e.subdivision_name}</p>
                      <p className="text-xs text-slate-500">{e.division_name}</p>
                    </Td>
                    <Td align="right" className="tabular-nums whitespace-nowrap">
                      {rupiah(e.base_salary)}
                    </Td>
                    <Td>{e.status === "active" ? <Badge tone="green">Aktif</Badge> : <Badge>Nonaktif</Badge>}</Td>
                    <Td align="right" className="whitespace-nowrap">
                      <IconButton label={`Ubah ${e.full_name}`} onClick={() => setEditing(e)}>
                        <Pencil className="size-4" />
                      </IconButton>
                      <IconButton label={`Hapus ${e.full_name}`} onClick={() => setDeleting(e)} className="hover:text-red-600">
                        <Trash2 className="size-4" />
                      </IconButton>
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
            <Pagination page={page} perPage={PER_PAGE} total={list.data.meta.total} onPage={setPage} />
          </>
        )}
      </Card>

      {editing !== null && <EmployeeForm employee={editing === "new" ? null : editing} onClose={() => setEditing(null)} />}

      <ConfirmDialog
        open={deleting !== null}
        title="Hapus data Pejuang?"
        message={
          <>
            <strong className="text-slate-800">{deleting?.full_name}</strong> ({deleting?.nip}) akan dihapus dari daftar. Riwayat presensi dan upah tetap
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
