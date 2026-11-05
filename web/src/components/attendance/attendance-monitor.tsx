"use client";

import { RefreshCw } from "lucide-react";
import dynamic from "next/dynamic";
import { useEffect, useMemo, useState } from "react";
import type { MonitorPoint } from "@/components/attendance/attendance-monitor-map";
import { IconButton } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/form";
import { Badge, Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { Pagination, Table, Td, Th } from "@/components/ui/table";
import { dateTime, number, TZ, todayWib } from "@/lib/format";
import { useAttendanceMonitor, useDivisions, useOfficeLocations } from "@/lib/hooks";
import type { AttendanceMonitorItem } from "@/lib/types";

const AttendanceMonitorMap = dynamic(() => import("@/components/attendance/attendance-monitor-map"), {
  ssr: false,
  loading: () => <Loading label="Memuat peta..." />,
});

const PER_PAGE = 20;

type State = "present" | "late" | "review" | "absent" | "leave" | "waiting" | "off";

const STATE: Record<State, { label: string; tone: "green" | "amber" | "red" | "gray"; color?: string }> = {
  present: { label: "Hadir", tone: "green", color: "#15803d" },
  late: { label: "Telat", tone: "amber", color: "#b45309" },
  review: { label: "Perlu ditinjau", tone: "red", color: "#b91c1c" },
  absent: { label: "Tidak hadir", tone: "red" },
  leave: { label: "Cuti", tone: "gray" },
  waiting: { label: "Belum absen", tone: "gray" },
  off: { label: "Tidak terjadwal", tone: "gray" },
};

const WORK_MODE: Record<string, string> = { onsite: "Kantor", wfa: "WFA", field: "Lapangan" };

const stateOf = (a: AttendanceMonitorItem): State => a.status ?? (a.scheduled ? "waiting" : "off");

const timeOf = (iso: string | null) => (iso ? new Date(iso).toLocaleTimeString("id-ID", { timeZone: TZ, hour: "2-digit", minute: "2-digit" }) : "-");

// Titik terakhir yang diketahui: titik pulang bila sudah absen pulang.
function pointOf(a: AttendanceMonitorItem): MonitorPoint | null {
  const color = STATE[stateOf(a)].color;
  const latitude = a.out_latitude ?? a.in_latitude;
  const longitude = a.out_longitude ?? a.in_longitude;
  if (!color || latitude === null || longitude === null) return null;
  const when = a.clock_out_at ? `Pulang ${timeOf(a.clock_out_at)}` : `Masuk ${timeOf(a.clock_in_at)}`;
  const place = a.work_mode ? (a.location_name ?? WORK_MODE[a.work_mode]) : "";
  return { id: a.employee_id, name: a.employee_name, detail: [STATE[stateOf(a)].label, when, place].filter(Boolean).join(" · "), color, latitude, longitude };
}

function Tile({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-md bg-slate-50 px-4 py-3">
      <p className="text-xs text-slate-500">{label}</p>
      <p className="mt-0.5 text-lg font-semibold text-slate-900 tabular-nums">{number(value)}</p>
    </div>
  );
}

export function AttendanceMonitor() {
  const today = todayWib();
  const [date, setDate] = useState(today);
  const [divisionId, setDivisionId] = useState("");
  const [state, setState] = useState("");
  const [page, setPage] = useState(1);
  const divisions = useDivisions();
  const offices = useOfficeLocations();
  const list = useAttendanceMonitor({ date, division_id: divisionId }, date === today);
  useEffect(() => setPage(1), [date, divisionId, state]);

  const rows = useMemo(() => list.data ?? [], [list.data]);
  const count = (s: State) => rows.filter((a) => stateOf(a) === s).length;
  const shown = useMemo(() => (state ? rows.filter((a) => stateOf(a) === state) : rows), [rows, state]);
  const points = useMemo(() => shown.map(pointOf).filter((p): p is MonitorPoint => p !== null), [shown]);
  const areas = useMemo(() => (offices.data ?? []).filter((o) => o.is_active), [offices.data]);
  const noPoint = shown.filter((a) => a.status && !pointOf(a) && STATE[stateOf(a)].color).length;
  const pageRows = shown.slice((page - 1) * PER_PAGE, page * PER_PAGE);

  return (
    <>
      <PageHeader
        title="Monitor Absen"
        description={date === today ? "Kondisi absen semua Pejuang hari ini, diperbarui otomatis tiap 60 detik." : "Kondisi absen semua Pejuang pada tanggal ini."}
      />
      <Card bodyClassName="">
        <div className="flex flex-wrap items-center gap-3 border-b border-slate-100 p-4">
          <Input type="date" value={date} max={today} onChange={(e) => setDate(e.target.value || today)} aria-label="Tanggal kerja" className="w-full sm:w-auto" />
          <Select value={divisionId} onChange={(e) => setDivisionId(e.target.value)} aria-label="Filter divisi" className="w-full sm:w-auto sm:min-w-44">
            <option value="">Semua divisi</option>
            {divisions.data?.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
              </option>
            ))}
          </Select>
          <Select value={state} onChange={(e) => setState(e.target.value)} aria-label="Filter status" className="w-full sm:w-auto sm:min-w-44">
            <option value="">Semua status</option>
            {(Object.keys(STATE) as State[]).map((s) => (
              <option key={s} value={s}>
                {STATE[s].label}
              </option>
            ))}
          </Select>
          <div className="ml-auto flex items-center gap-2 text-xs text-slate-500">
            {list.dataUpdatedAt > 0 && <span>Diperbarui {dateTime(new Date(list.dataUpdatedAt).toISOString())}</span>}
            <IconButton label="Perbarui sekarang" onClick={() => list.refetch()}>
              <RefreshCw className={`size-4 ${list.isFetching ? "animate-spin" : ""}`} />
            </IconButton>
          </div>
        </div>

        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : rows.length === 0 ? (
          <EmptyState title="Tidak ada Pejuang" description="Ubah tanggal atau saringan divisi." />
        ) : (
          <>
            <div className="grid grid-cols-2 gap-3 border-b border-slate-100 p-4 md:grid-cols-5">
              <Tile label="Total Pejuang" value={rows.length} />
              <Tile label="Hadir" value={count("present")} />
              <Tile label="Telat" value={count("late")} />
              <Tile label="Perlu ditinjau" value={count("review")} />
              <Tile label="Belum absen" value={count("waiting")} />
            </div>

            <div className="border-b border-slate-100 p-4">
              <AttendanceMonitorMap points={points} offices={areas} fitKey={`${date}|${divisionId}|${state}`} />
              <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-slate-600">
                {(["present", "late", "review"] as const).map((s) => (
                  <span key={s} className="inline-flex items-center gap-1.5">
                    <span className="size-2.5 rounded-full" style={{ background: STATE[s].color }} aria-hidden />
                    {STATE[s].label}
                  </span>
                ))}
                <span>Lingkaran putus-putus adalah area kantor.</span>
                <span className="sm:hidden">Geser peta dengan dua jari.</span>
              </div>
              <p className="mt-1 text-xs text-slate-500">
                Titik adalah lokasi saat absen terakhir, bukan posisi terkini.
                {noPoint > 0 && ` ${number(noPoint)} Pejuang hadir tanpa titik karena datanya simulasi.`}
              </p>
            </div>

            {shown.length === 0 ? (
              <EmptyState title="Tidak ada Pejuang dengan status ini" description="Ubah saringan status." />
            ) : (
              <>
                <Table>
                  <thead>
                    <tr>
                      <Th>Pejuang</Th>
                      <Th>Status</Th>
                      <Th>Masuk</Th>
                      <Th>Pulang</Th>
                      <Th>Tempat</Th>
                    </tr>
                  </thead>
                  <tbody className={list.isPlaceholderData ? "opacity-60" : ""}>
                    {pageRows.map((a) => (
                      <tr key={a.employee_id} className="hover:bg-slate-50">
                        <Td>
                          <p className="font-medium text-slate-900">{a.employee_name}</p>
                          <p className="text-xs text-slate-500">
                            {a.nip} · {a.subdivision_name}
                          </p>
                        </Td>
                        <Td>
                          <Badge tone={STATE[stateOf(a)].tone}>{STATE[stateOf(a)].label}</Badge>
                          {a.late_minutes > 0 && <p className="mt-1 text-xs text-slate-500">{a.late_minutes} menit</p>}
                        </Td>
                        <Td className="tabular-nums whitespace-nowrap">{timeOf(a.clock_in_at)}</Td>
                        <Td className="tabular-nums whitespace-nowrap">{timeOf(a.clock_out_at)}</Td>
                        <Td>
                          {a.work_mode ? WORK_MODE[a.work_mode] : "-"}
                          {a.location_name && <p className="text-xs text-slate-500">{a.location_name}</p>}
                        </Td>
                      </tr>
                    ))}
                  </tbody>
                </Table>
                <Pagination page={page} perPage={PER_PAGE} total={shown.length} onPage={setPage} />
              </>
            )}
          </>
        )}
      </Card>
    </>
  );
}
