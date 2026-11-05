"use client";

import { Download, Info } from "lucide-react";
import { useMemo, useState } from "react";
import { currentPeriod, PeriodFilterBar } from "@/components/reports/period-filter";
import { Button } from "@/components/ui/button";
import { Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { Table, Td, Th } from "@/components/ui/table";
import { downloadCsv } from "@/lib/csv";
import { MONTHS, number, percent } from "@/lib/format";
import { useAttendanceReport, type PeriodFilter } from "@/lib/hooks";

function Summary({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md bg-slate-50 px-4 py-3">
      <p className="text-xs text-slate-500">{label}</p>
      <p className="mt-0.5 text-lg font-semibold text-slate-900 tabular-nums">{value}</p>
    </div>
  );
}

export default function AttendanceReportPage() {
  const [period, setPeriod] = useState<PeriodFilter>(() => currentPeriod());
  const report = useAttendanceReport(period);
  const rows = useMemo(() => report.data ?? [], [report.data]);

  const totals = useMemo(() => {
    const t = rows.reduce(
      (a, r) => ({ days: a.days + r.work_days, present: a.present + r.present, late: a.late + r.late, leave: a.leave + r.leave, absent: a.absent + r.absent }),
      { days: 0, present: 0, late: 0, leave: 0, absent: 0 },
    );
    return { ...t, rate: t.days ? t.present / t.days : 0 };
  }, [rows]);

  const label = `${MONTHS[period.month - 1]} ${period.year}`;

  function exportCsv() {
    downloadCsv(
      `rekap-presensi-${period.year}-${String(period.month).padStart(2, "0")}.csv`,
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
          <Button variant="secondary" onClick={exportCsv} disabled={!rows.length}>
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
          <PeriodFilterBar value={period} onChange={setPeriod} />
          <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
            <Summary label="Rata-rata kehadiran" value={percent(totals.rate)} />
            <Summary label="Total hadir" value={number(totals.present)} />
            <Summary label="Total telat" value={number(totals.late)} />
            <Summary label="Total cuti/izin" value={number(totals.leave)} />
            <Summary label="Total alpha" value={number(totals.absent)} />
          </div>
        </div>
        {report.isPending ? (
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
                <Th align="right">Hari kerja</Th>
                <Th align="right">Hadir</Th>
                <Th align="right">Telat</Th>
                <Th align="right">Cuti/Izin</Th>
                <Th align="right">Alpha</Th>
                <Th align="right">Menit telat</Th>
                <Th align="right">Kehadiran</Th>
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
                  <Td align="right" className="tabular-nums">{r.work_days}</Td>
                  <Td align="right" className="tabular-nums">{r.present}</Td>
                  <Td align="right" className={`tabular-nums ${r.late > 3 ? "font-medium text-amber-700" : ""}`}>{r.late}</Td>
                  <Td align="right" className="tabular-nums">{r.leave}</Td>
                  <Td align="right" className={`tabular-nums ${r.absent > 0 ? "font-medium text-red-700" : ""}`}>{r.absent}</Td>
                  <Td align="right" className="tabular-nums">{number(r.late_minutes)}</Td>
                  <Td align="right" className="tabular-nums">{r.work_days ? percent(r.present / r.work_days) : "-"}</Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card>
    </>
  );
}
