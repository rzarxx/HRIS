"use client";

import { ChevronRight, Download, Inbox, Search } from "lucide-react";
import Link from "next/link";
import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input, Select } from "@/components/ui/form";
import { Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { Modal } from "@/components/ui/modal";
import { Table, Td, Th } from "@/components/ui/table";
import { downloadCsv } from "@/lib/csv";
import { dateLong, dateRange, dateTime, initials, number, timeAgo, todayWib } from "@/lib/format";
import { useAdminLeaves, useMe, usePendingApprovals } from "@/lib/hooks";
import type { LeaveRequest, RequestStatus } from "@/lib/types";
import { ApprovalTimeline } from "./approval-timeline";
import { APPROVER_ROLE, REQUEST_STATUS, RequestStatusBadge, approverName, currentStep } from "./request-status";

const TABS: (RequestStatus | "")[] = ["", "pending", "approved", "rejected", "cancelled"];

function yearRange() {
  const year = todayWib().slice(0, 4);
  return { from: `${year}-01-01`, to: `${year}-12-31` };
}

/** Keterangan posisi pengajuan: di langkah mana dan siapa yang memegangnya. */
function progressText(r: LeaveRequest) {
  const step = currentStep(r.approvals);
  if (r.status === "pending" && step) return `Di ${APPROVER_ROLE[step.approver_role]} · ${approverName(step)}`;
  if (r.decided_at) return `${r.status === "cancelled" ? "Dibatalkan" : "Diputuskan"} ${dateTime(r.decided_at)}`;
  return "";
}

export function LeaveList() {
  const me = useMe();
  const [range, setRange] = useState(yearRange);
  const [status, setStatus] = useState<RequestStatus | "">("pending");
  const [type, setType] = useState("");
  const [q, setQ] = useState("");
  const [detail, setDetail] = useState<LeaveRequest | null>(null);
  const list = useAdminLeaves(range);
  const pending = usePendingApprovals(me.data?.permissions.includes("approval:act") ?? false);

  const all = useMemo(() => list.data ?? [], [list.data]);
  const types = useMemo(() => [...new Map(all.map((r) => [r.leave_type.id, r.leave_type.name])).entries()], [all]);
  const counts = useMemo(() => {
    const c: Record<string, number> = { "": all.length };
    for (const r of all) c[r.status] = (c[r.status] ?? 0) + 1;
    return c;
  }, [all]);
  const approvedDays = useMemo(() => all.filter((r) => r.status === "approved").reduce((n, r) => n + r.days, 0), [all]);

  const rows = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return all.filter(
      (r) =>
        (!status || r.status === status) &&
        (!type || r.leave_type.id === type) &&
        (!needle || `${r.employee_name ?? ""} ${r.subdivision_name ?? ""}`.toLowerCase().includes(needle)),
    );
  }, [all, status, type, q]);

  const myPending = pending.data?.length ?? 0;

  function exportCsv() {
    downloadCsv(
      `cuti-izin-${range.from}-${range.to}.csv`,
      ["Pejuang", "Subdiv", "Jenis", "Mulai", "Selesai", "Hari kerja", "Status", "Alasan", "Diajukan", "Diputuskan"],
      rows.map((r) => [
        r.employee_name ?? "",
        r.subdivision_name ?? "",
        r.leave_type.name,
        r.start_date,
        r.end_date,
        r.days,
        REQUEST_STATUS[r.status].label,
        r.reason,
        dateTime(r.created_at),
        r.decided_at ? dateTime(r.decided_at) : "",
      ]),
    );
  }

  return (
    <>
      <PageHeader
        title="Cuti & Izin"
        description="Pengajuan cuti dan izin Pejuang dalam cakupan Anda, lengkap dengan jejak persetujuannya."
        actions={
          <>
            {myPending > 0 && (
              <Link
                href="/pengajuan/persetujuan"
                className="inline-flex h-9 items-center gap-2 rounded-md bg-amber-50 px-3.5 text-sm font-medium text-amber-900 ring-1 ring-amber-200 transition-colors hover:bg-amber-100"
              >
                <Inbox className="size-4" aria-hidden />
                {myPending} menunggu keputusan Anda
              </Link>
            )}
            <Button variant="secondary" onClick={exportCsv} disabled={rows.length === 0}>
              <Download className="size-4" aria-hidden /> Unduh CSV
            </Button>
          </>
        }
      />

      <div className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label="Menunggu" value={counts.pending ?? 0} tone="text-amber-700" />
        <Stat label="Disetujui" value={counts.approved ?? 0} tone="text-emerald-700" />
        <Stat label="Ditolak" value={counts.rejected ?? 0} tone="text-red-700" />
        <Stat label="Hari cuti disetujui" value={approvedDays} tone="text-slate-900" />
      </div>

      <Card bodyClassName="">
        <div className="flex gap-1 overflow-x-auto overflow-y-hidden [scrollbar-width:none] border-b border-slate-100 px-2 pt-2" role="tablist" aria-label="Status pengajuan">
          {TABS.map((t) => {
            const active = status === t;
            return (
              <button
                key={t || "all"}
                type="button"
                role="tab"
                aria-selected={active}
                onClick={() => setStatus(t)}
                className={`-mb-px flex shrink-0 items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-medium transition-colors ${
                  active ? "border-primary-600 text-primary-700" : "border-transparent text-slate-500 hover:text-slate-800"
                }`}
              >
                {t ? REQUEST_STATUS[t].label : "Semua"}
                <span className={`rounded-full px-1.5 text-xs ${active ? "bg-primary-50 text-primary-700" : "bg-slate-100 text-slate-600"}`}>{counts[t] ?? 0}</span>
              </button>
            );
          })}
        </div>

        <div className="grid grid-cols-2 gap-3 border-b border-slate-100 p-4 lg:grid-cols-4">
          <Field label="Cari" htmlFor="leave_q" className="col-span-2 lg:col-span-1">
            <div className="relative">
              <Search className="pointer-events-none absolute top-2.5 left-2.5 size-4 text-slate-400" aria-hidden />
              <Input id="leave_q" value={q} onChange={(e) => setQ(e.target.value)} placeholder="Nama atau subdiv" className="w-full pl-8" />
            </div>
          </Field>
          <Field label="Jenis" htmlFor="leave_type" className="col-span-2 lg:col-span-1">
            <Select id="leave_type" value={type} onChange={(e) => setType(e.target.value)}>
              <option value="">Semua jenis</option>
              {types.map(([id, name]) => (
                <option key={id} value={id}>
                  {name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Mulai dari" htmlFor="leave_from">
            <Input id="leave_from" type="date" value={range.from} max={range.to} onChange={(e) => e.target.value && setRange((r) => ({ ...r, from: e.target.value }))} />
          </Field>
          <Field label="Sampai" htmlFor="leave_to">
            <Input id="leave_to" type="date" value={range.to} min={range.from} onChange={(e) => e.target.value && setRange((r) => ({ ...r, to: e.target.value }))} />
          </Field>
        </div>

        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : rows.length === 0 ? (
          <EmptyState
            title={all.length === 0 ? "Belum ada pengajuan pada rentang ini" : "Tidak ada yang cocok dengan filter"}
            description={all.length === 0 ? "Pengajuan dari aplikasi HP Pejuang akan muncul di sini." : "Ubah tab status, jenis, atau kata pencarian."}
          />
        ) : (
          <div className={list.isPlaceholderData ? "opacity-60 transition-opacity" : "transition-opacity"}>
            {/* Ponsel: kartu yang bisa diketuk. Layar lebar: tabel. */}
            <ul className="divide-y divide-slate-100 sm:hidden">
              {rows.map((r) => (
                <li key={r.id}>
                  <button type="button" onClick={() => setDetail(r)} className="flex w-full items-start gap-3 px-4 py-3 text-left active:bg-slate-50">
                    <Avatar name={r.employee_name ?? "?"} />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-start justify-between gap-2">
                        <p className="truncate font-medium text-slate-900">{r.employee_name}</p>
                        <RequestStatusBadge status={r.status} />
                      </div>
                      <p className="text-sm text-slate-700">
                        {r.leave_type.name} · {number(r.days)} hari
                      </p>
                      <p className="text-xs text-slate-500">{dateRange(r.start_date, r.end_date)}</p>
                      {progressText(r) && <p className="mt-0.5 truncate text-xs text-slate-500">{progressText(r)}</p>}
                    </div>
                    <ChevronRight className="mt-2 size-4 shrink-0 text-slate-300" aria-hidden />
                  </button>
                </li>
              ))}
            </ul>
            <div className="hidden sm:block">
              <Table actions="sm">
                <thead>
                  <tr>
                    <Th>Pejuang</Th>
                    <Th>Jenis</Th>
                    <Th>Tanggal</Th>
                    <Th align="right">Hari</Th>
                    <Th>Status</Th>
                    <Th>
                      <span className="sr-only">Aksi</span>
                    </Th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((r) => (
                    <tr key={r.id} className="cursor-pointer hover:bg-slate-50" onClick={() => setDetail(r)}>
                      <Td>
                        <div className="flex items-center gap-2.5">
                          <Avatar name={r.employee_name ?? "?"} />
                          <div className="min-w-0">
                            <p className="font-medium text-slate-900">{r.employee_name}</p>
                            <p className="text-xs text-slate-500">{r.subdivision_name}</p>
                          </div>
                        </div>
                      </Td>
                      <Td>{r.leave_type.name}</Td>
                      <Td className="whitespace-nowrap">
                        {dateRange(r.start_date, r.end_date)}
                        <p className="text-xs text-slate-500">Diajukan {timeAgo(r.created_at)}</p>
                      </Td>
                      <Td align="right">{number(r.days)}</Td>
                      <Td>
                        <RequestStatusBadge status={r.status} />
                        {progressText(r) && <p className="mt-0.5 max-w-56 truncate text-xs text-slate-500">{progressText(r)}</p>}
                      </Td>
                      <Td align="right">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={(e) => {
                            e.stopPropagation();
                            setDetail(r);
                          }}
                          aria-label={`Lihat detail pengajuan ${r.employee_name}`}
                        >
                          Detail
                        </Button>
                      </Td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            </div>
            <p className="px-4 py-3 text-xs text-slate-500">
              Menampilkan {number(rows.length)} dari {number(all.length)} pengajuan, {dateLong(range.from)} sampai {dateLong(range.to)}.
            </p>
          </div>
        )}
      </Card>

      {detail && (
        <LeaveDetailModal
          leave={detail}
          canDecide={pending.data?.some((a) => a.request_type === "leave" && a.request_id === detail.id) ?? false}
          onClose={() => setDetail(null)}
        />
      )}
    </>
  );
}

function Stat({ label, value, tone }: { label: string; value: number; tone: string }) {
  return (
    <div className="rounded-[var(--radius-card)] border border-slate-200 bg-white px-4 py-3">
      <p className="text-xs font-medium text-slate-500">{label}</p>
      <p className={`mt-0.5 text-2xl font-semibold tabular-nums ${tone}`}>{number(value)}</p>
    </div>
  );
}

export function Avatar({ name }: { name: string }) {
  return (
    <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-primary-50 text-xs font-semibold text-primary-700" aria-hidden>
      {initials(name)}
    </span>
  );
}

function LeaveDetailModal({ leave, canDecide, onClose }: { leave: LeaveRequest; canDecide: boolean; onClose: () => void }) {
  return (
    <Modal
      open
      title={`${leave.leave_type.name} · ${leave.employee_name ?? ""}`}
      onClose={onClose}
      footer={
        <>
          {leave.status === "pending" && canDecide && (
            <Link href="/pengajuan/persetujuan" className="mr-auto inline-flex h-9 items-center text-sm font-medium text-primary-700 hover:underline">
              Putuskan di kotak persetujuan
            </Link>
          )}
          <Button variant="secondary" onClick={onClose}>
            Tutup
          </Button>
        </>
      }
    >
      <div className="mb-4 flex items-center justify-between gap-3 rounded-lg bg-slate-50 px-4 py-3">
        <div>
          <p className="text-lg font-semibold text-slate-900">{dateRange(leave.start_date, leave.end_date)}</p>
          <p className="text-sm text-slate-600">{number(leave.days)} hari kerja</p>
        </div>
        <RequestStatusBadge status={leave.status} />
      </div>
      <dl className="grid grid-cols-3 gap-y-2 text-sm">
        <dt className="text-slate-500">Pejuang</dt>
        <dd className="col-span-2 text-slate-800">{leave.employee_name}</dd>
        <dt className="text-slate-500">Subdiv</dt>
        <dd className="col-span-2 text-slate-800">{leave.subdivision_name ?? "-"}</dd>
        <dt className="text-slate-500">Alasan</dt>
        <dd className="col-span-2 break-words whitespace-pre-line text-slate-800">{leave.reason}</dd>
        {leave.decided_at && (
          <>
            <dt className="text-slate-500">Diputuskan</dt>
            <dd className="col-span-2 text-slate-800">{dateTime(leave.decided_at)}</dd>
          </>
        )}
      </dl>
      <h3 className="mt-5 mb-3 text-sm font-semibold text-slate-800">Riwayat persetujuan</h3>
      <ApprovalTimeline steps={leave.approvals} requester={leave.employee_name ?? "Pejuang"} submittedAt={leave.created_at} />
    </Modal>
  );
}
