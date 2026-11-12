"use client";

import { Download, Info } from "lucide-react";
import { useMemo, useState } from "react";
import { currentPeriod, PeriodFilterBar } from "@/components/reports/period-filter";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/form";
import { Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { Table, Td, Th } from "@/components/ui/table";
import { downloadCsv } from "@/lib/csv";
import { MONTHS, number, percent } from "@/lib/format";
import { useAttendanceReport, type AttendanceReportFilter } from "@/lib/hooks";

function Summary({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md bg-slate-50 px-4 py-3">
      <p className="text-xs text-slate-500">{label}</p>
      <p className="mt-0.5 text-lg font-semibold text-slate-900 tabular-nums">{value}</p>
    </div>
  );
}

/** Menit kerja sebagai "8 j 30 m" untuk tabel. */
function hours(minutes: number) {
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return m ? `${number(h)} j ${m} m` : `${number(h)} j`;
}

/** Menit kerja sebagai jam desimal berkoma untuk Excel berlokal Indonesia. */
const hoursCsv = (minutes: number) => (minutes / 60).toFixed(2).replace(".", ",");

export default function AttendanceReportPage() {
  const [period, setPeriod] = useState<AttendanceReportFilter>(() => currentPeriod());
  const report = useAttendanceReport(period);
  const rows = useMemo(() => report.isPlaceholderData ? [] : report.data ?? [], [report.data, report.isPlaceholderData]);

  const totals = useMemo(() => {
    const t = rows.reduce(
      (a, r) => ({
        days: a.days + r.work_days, present: a.present + r.present, late: a.late + r.late, leave: a.leave + r.leave,
        absent: a.absent + r.absent, review: a.review + r.review, worked: a.worked + r.worked_minutes, missed: a.missed + r.missed_clock_out,
      }),
      { days: 0, present: 0, late: 0, leave: 0, absent: 0, review: 0, worked: 0, missed: 0 },
    );
    return { ...t, rate: t.days ? t.present / t.days : 0 };
  }, [rows]);

  // Jenis cuti yang muncul di periode ini, untuk kolom ekspor per jenis.
  const leaveTypes = useMemo(() => {
    const types = new Map<string, string>();
    for (const r of rows) for (const t of r.leave_by_type) types.set(t.code, t.name);
    return [...types].sort((a, b) => a[1].localeCompare(b[1], "id"));
  }, [rows]);

  const label = `${MONTHS[period.month - 1]} ${period.year}`;

  function exportCsv() {
    const base = `rekap-presensi-${period.year}-${String(period.month).padStart(2, "0")}`;
    if (period.work_mode) {
      downloadCsv(
        `${base}-${period.work_mode}.csv`,
        ["NIPY", "Nama", "Divisi", "Subdiv", "Hadir", "Telat", "Perlu ditinjau", "Lupa presensi pulang", "Jam kerja", "Total menit telat"],
        rows.map((r) => [r.nip, r.full_name, r.division, r.subdivision, r.present, r.late, r.review, r.missed_clock_out, hoursCsv(r.worked_minutes), r.late_minutes]),
      );
      return;
    }
    downloadCsv(
      `${base}.csv`,
      [
        "NIPY", "Nama", "Divisi", "Subdiv", "Hari kerja", "Hadir", "Telat", "Perlu ditinjau", "Cuti/Izin",
        "Cuti bersaldo", "Cuti tanpa saldo", "Cuti lainnya", "Cuti tidak dibayar", ...leaveTypes.map(([, name]) => `Cuti: ${name}`),
        "Alpha", "Lupa presensi pulang", "Jam kerja", "Total menit telat", "Kehadiran (%)",
      ],
      rows.map((r) => [
        r.nip, r.full_name, r.division, r.subdivision, r.work_days, r.present, r.late, r.review, r.leave,
        r.leave_quota, r.leave_non_quota, r.leave_other, r.leave_unpaid,
        ...leaveTypes.map(([code]) => r.leave_by_type.find((t) => t.code === code)?.days ?? 0),
        r.absent, r.missed_clock_out, hoursCsv(r.worked_minutes), r.late_minutes,
        (r.work_days ? (r.present / r.work_days) * 100 : 0).toFixed(1).replace(".", ","),
      ]),
    );
  }

  return (
    <>
      <PageHeader
        title="Rekap presensi"
        description={`Rekap kehadiran Pejuang periode ${label}.`}
        actions={
          <Button variant="secondary" onClick={exportCsv} disabled={!rows.length || report.isPlaceholderData || report.isError}>
            <Download className="size-4" /> Ekspor Excel (CSV)
          </Button>
        }
      />
      <div className="mb-4 flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
        <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden />
        Data presensi saat ini adalah simulasi. Presensi nyata dari aplikasi HP (geofence + verifikasi wajah) hadir di Fase 2-3.
      </div>
      <Card bodyClassName="">
        <div className="space-y-4 border-b border-slate-100 p-4">
          <PeriodFilterBar value={period} onChange={(value) => setPeriod({ ...value, work_mode: period.work_mode })} />
          <Select aria-label="Mode kerja" value={period.work_mode ?? ""} onChange={(e) => setPeriod({ ...period, work_mode: (e.target.value || undefined) as AttendanceReportFilter["work_mode"] })}>
            <option value="">Semua mode kerja</option>
            <option value="onsite">Di kantor</option>
            <option value="wfa">WFA</option>
            <option value="field">Dinas luar</option>
          </Select>
          {period.work_mode && <p className="text-xs text-slate-600">Hanya presensi masuk dengan mode ini yang dihitung. Cuti dan alpha tidak memiliki mode kerja yang dapat dipastikan.</p>}
          {totals.review > 0 && <p className="text-xs text-slate-600">Presensi yang perlu ditinjau tetap dihitung sebagai hari kerja, tetapi belum dihitung hadir sampai diputuskan di Review Presensi.</p>}
          <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
            {!period.work_mode && <Summary label="Rata-rata kehadiran" value={percent(totals.rate)} />}
            <Summary label="Total hadir" value={number(totals.present)} />
            <Summary label="Total telat" value={number(totals.late)} />
            <Summary label="Perlu ditinjau" value={number(totals.review)} />
            {!period.work_mode && <Summary label="Total cuti/izin" value={number(totals.leave)} />}
            {!period.work_mode && <Summary label="Total alpha" value={number(totals.absent)} />}
            <Summary label="Lupa presensi pulang" value={number(totals.missed)} />
            <Summary label="Total jam kerja" value={hours(totals.worked)} />
          </div>
        </div>
        {report.isPending || report.isPlaceholderData ? (
          <Loading />
        ) : report.isError ? (
          <ErrorState message={report.error.message} onRetry={() => report.refetch()} />
        ) : totals.days === 0 ? (
          <EmptyState title={`Belum ada data presensi ${label}`} description="Pilih periode lain." />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>NIPY</Th>
                <Th>Nama</Th>
                <Th>Subdiv</Th>
                {!period.work_mode && <Th align="right">Hari kerja</Th>}
                <Th align="right">Hadir</Th>
                <Th align="right">Telat</Th>
                <Th align="right">Perlu ditinjau</Th>
                {!period.work_mode && <Th align="right">Cuti/Izin</Th>}
                {!period.work_mode && <Th align="right">Alpha</Th>}
                <Th align="right">Lupa pulang</Th>
                <Th align="right">Jam kerja</Th>
                <Th align="right">Menit telat</Th>
                {!period.work_mode && <Th align="right">Kehadiran</Th>}
              </tr>
            </thead>
            <tbody className={report.isPlaceholderData ? "opacity-60" : ""}>
              {rows.map((r) => (
                <tr key={r.employee_id} className="hover:bg-slate-50">
                  <Td className="font-mono text-xs">{r.nip}</Td>
                  <Td className="font-medium whitespace-nowrap text-slate-900">{r.full_name}</Td>
                  <Td className="whitespace-nowrap">
                    {r.subdivision}
                    <span className="block text-xs text-slate-500">{r.division}</span>
                  </Td>
                  {!period.work_mode && <Td align="right" className="tabular-nums">{r.work_days}</Td>}
                  <Td align="right" className="tabular-nums">{r.present}</Td>
                  <Td align="right" className={`tabular-nums ${r.late > 3 ? "font-medium text-amber-700" : ""}`}>{r.late}</Td>
                  <Td align="right" className={`tabular-nums ${r.review > 0 ? "font-medium text-amber-700" : ""}`}>{r.review}</Td>
                  {!period.work_mode && (
                    <Td align="right" className="tabular-nums whitespace-nowrap">
                      {r.leave}
                      {r.leave > 0 && (
                        <span className="block text-xs text-slate-500" title="Bersaldo / tanpa saldo / tanpa pengajuan disetujui">
                          {r.leave_quota} saldo · {r.leave_non_quota} non · {r.leave_other} lain
                        </span>
                      )}
                    </Td>
                  )}
                  {!period.work_mode && <Td align="right" className={`tabular-nums ${r.absent > 0 ? "font-medium text-red-700" : ""}`}>{r.absent}</Td>}
                  <Td align="right" className={`tabular-nums ${r.missed_clock_out > 0 ? "font-medium text-amber-700" : ""}`}>{r.missed_clock_out}</Td>
                  <Td align="right" className="tabular-nums whitespace-nowrap">{hours(r.worked_minutes)}</Td>
                  <Td align="right" className="tabular-nums">{number(r.late_minutes)}</Td>
                  {!period.work_mode && <Td align="right" className="tabular-nums">{r.work_days ? percent(r.present / r.work_days) : "-"}</Td>}
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card>
    </>
  );
}
