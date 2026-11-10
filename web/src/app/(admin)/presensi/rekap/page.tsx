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

export default function AttendanceReportPage() {
  const [period, setPeriod] = useState<AttendanceReportFilter>(() => currentPeriod());
  const report = useAttendanceReport(period);
  const rows = useMemo(() => report.isPlaceholderData ? [] : report.data ?? [], [report.data, report.isPlaceholderData]);

  const totals = useMemo(() => {
    const t = rows.reduce(
      (a, r) => ({ days: a.days + r.work_days, present: a.present + r.present, late: a.late + r.late, leave: a.leave + r.leave, absent: a.absent + r.absent }),
      { days: 0, present: 0, late: 0, leave: 0, absent: 0 },
    );
    return { ...t, rate: t.days ? t.present / t.days : 0 };
  }, [rows]);

  const label = `${MONTHS[period.month - 1]} ${period.year}`;

  function exportCsv() {
    if (period.work_mode) {
      downloadCsv(
        `rekap-presensi-${period.year}-${String(period.month).padStart(2, "0")}-${period.work_mode}.csv`,
        ["NIPY", "Nama", "Divisi", "Subdiv", "Hadir", "Telat", "Total menit telat"],
        rows.map((r) => [r.nip, r.full_name, r.division, r.subdivision, r.present, r.late, r.late_minutes]),
      );
      return;
    }
    downloadCsv(
      `rekap-presensi-${period.year}-${String(period.month).padStart(2, "0")}${period.work_mode ? `-${period.work_mode}` : ""}.csv`,
      ["NIPY", "Nama", "Divisi", "Subdiv", "Hari kerja", "Hadir", "Telat", "Cuti/Izin", "Alpha", "Total menit telat", "Kehadiran (%)"],
      rows.map((r) => [r.nip, r.full_name, r.division, r.subdivision, r.work_days, r.present, r.late, r.leave, r.absent, r.late_minutes, (r.work_days ? (r.present / r.work_days) * 100 : 0).toFixed(1).replace(".", ",")]),
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
          <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
            {!period.work_mode && <Summary label="Rata-rata kehadiran" value={percent(totals.rate)} />}
            <Summary label="Total hadir" value={number(totals.present)} />
            <Summary label="Total telat" value={number(totals.late)} />
            {!period.work_mode && <Summary label="Total cuti/izin" value={number(totals.leave)} />}
            {!period.work_mode && <Summary label="Total alpha" value={number(totals.absent)} />}
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
                {!period.work_mode && <Th align="right">Cuti/Izin</Th>}
                {!period.work_mode && <Th align="right">Alpha</Th>}
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
                  {!period.work_mode && <Td align="right" className="tabular-nums">{r.leave}</Td>}
                  {!period.work_mode && <Td align="right" className={`tabular-nums ${r.absent > 0 ? "font-medium text-red-700" : ""}`}>{r.absent}</Td>}
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
