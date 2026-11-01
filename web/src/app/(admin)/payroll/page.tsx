"use client";

import { Download, Info } from "lucide-react";
import { useMemo, useState } from "react";
import { currentPeriod, PeriodFilterBar } from "@/components/reports/period-filter";
import { Button } from "@/components/ui/button";
import { Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { Table, Td, Th } from "@/components/ui/table";
import { downloadCsv } from "@/lib/csv";
import { MONTHS, rupiah } from "@/lib/format";
import { usePayrollReport, type PeriodFilter } from "@/lib/hooks";

export default function PayrollReportPage() {
  // Laporan upah lazimnya untuk bulan yang sudah selesai
  const [period, setPeriod] = useState<PeriodFilter>(() => currentPeriod(-1));
  const report = usePayrollReport(period);
  const rows = useMemo(() => report.data?.data ?? [], [report.data]);
  const rules = report.data?.meta.rules;

  const totals = useMemo(
    () =>
      rows.reduce(
        (a, r) => ({
          base: a.base + r.base_salary,
          absent: a.absent + r.absent_deduction,
          late: a.late + r.late_deduction,
          net: a.net + r.net,
        }),
        { base: 0, absent: 0, late: 0, net: 0 },
      ),
    [rows],
  );

  const label = `${MONTHS[period.month - 1]} ${period.year}`;

  function exportCsv() {
    downloadCsv(
      `laporan-upah-${period.year}-${String(period.month).padStart(2, "0")}.csv`,
      ["NIP", "Nama", "Divisi", "Subdiv", "Gaji pokok", "Alpha (hari)", "Telat (kali)", "Potongan alpha", "Potongan telat", "Upah bersih"],
      rows.map((r) => [r.nip, r.full_name, r.division, r.subdivision, r.base_salary, r.absent, r.late, r.absent_deduction, r.late_deduction, r.net]),
    );
  }

  return (
    <>
      <PageHeader
        title="Laporan upah"
        description={`Perhitungan upah berdasarkan presensi periode ${label}.`}
        actions={
          <Button variant="secondary" onClick={exportCsv} disabled={!rows.length}>
            <Download className="size-4" /> Ekspor Excel (CSV)
          </Button>
        }
      />
      {rules && (
        <div className="mb-4 flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
          <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden />
          {rules.note}
        </div>
      )}
      <Card bodyClassName="">
        <div className="space-y-4 border-b border-slate-100 p-4">
          <PeriodFilterBar value={period} onChange={setPeriod} />
          <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
            {[
              ["Total gaji pokok", totals.base],
              ["Potongan alpha", totals.absent],
              ["Potongan telat", totals.late],
              ["Total upah bersih", totals.net],
            ].map(([k, v]) => (
              <div key={k} className="rounded-md bg-slate-50 px-4 py-3">
                <p className="text-xs text-slate-500">{k}</p>
                <p className="mt-0.5 text-lg font-semibold text-slate-900 tabular-nums">{rupiah(v as number)}</p>
              </div>
            ))}
          </div>
        </div>
        {report.isPending ? (
          <Loading />
        ) : report.isError ? (
          <ErrorState message={report.error.message} onRetry={() => report.refetch()} />
        ) : rows.length === 0 ? (
          <EmptyState title="Tidak ada data" description="Pilih periode atau divisi lain." />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>NIP</Th>
                <Th>Nama</Th>
                <Th>Subdiv</Th>
                <Th align="right">Gaji pokok</Th>
                <Th align="right">Alpha</Th>
                <Th align="right">Telat</Th>
                <Th align="right">Pot. alpha</Th>
                <Th align="right">Pot. telat</Th>
                <Th align="right">Upah bersih</Th>
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
                  <Td align="right" className="tabular-nums whitespace-nowrap">{rupiah(r.base_salary)}</Td>
                  <Td align="right" className="tabular-nums">{r.absent}</Td>
                  <Td align="right" className="tabular-nums">{r.late}</Td>
                  <Td align="right" className={`tabular-nums whitespace-nowrap ${r.absent_deduction ? "text-red-700" : "text-slate-400"}`}>
                    {r.absent_deduction ? `−${rupiah(r.absent_deduction)}` : "-"}
                  </Td>
                  <Td align="right" className={`tabular-nums whitespace-nowrap ${r.late_deduction ? "text-red-700" : "text-slate-400"}`}>
                    {r.late_deduction ? `−${rupiah(r.late_deduction)}` : "-"}
                  </Td>
                  <Td align="right" className="font-semibold tabular-nums whitespace-nowrap text-slate-900">{rupiah(r.net)}</Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card>
    </>
  );
}
