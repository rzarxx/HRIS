"use client";

import { ChevronRight, Plus } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { PeriodStatusBadge, periodLabel } from "@/components/payroll/status";
import { Button } from "@/components/ui/button";
import { Field, Select } from "@/components/ui/form";
import { Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { Modal } from "@/components/ui/modal";
import { Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { dateLong, MONTHS, rupiah, todayWib } from "@/lib/format";
import { usePayrollPeriods, useSave } from "@/lib/hooks";
import type { PayrollPeriod } from "@/lib/types";

export default function PayrollPeriodsPage() {
  const list = usePayrollPeriods();
  const router = useRouter();
  const [creating, setCreating] = useState(false);
  const thisYear = Number(todayWib().slice(0, 4));
  const thisMonth = Number(todayWib().slice(5, 7));
  const [form, setForm] = useState({ year: thisYear, month: thisMonth });
  const [errors, setErrors] = useState<Record<string, string>>({});

  const create = useSave((v: { year: number; month: number }) => api.post<{ data: PayrollPeriod }>("/admin/payroll/periods", v), {
    success: "Periode ubar dibuat.",
    onDone: (res) => {
      setCreating(false);
      router.push(`/payroll/periode?id=${res.data.id}`);
    },
    onFieldErrors: setErrors,
  });

  function open() {
    setErrors({});
    setForm({ year: thisYear, month: thisMonth });
    setCreating(true);
  }

  return (
    <>
      <PageHeader
        title="Periode ubar"
        description="Hitung ubar per periode dari presensi dan komponen ubar, lalu ajukan untuk disetujui SDM lain sebelum slip terbit."
        actions={
          <Button onClick={open}>
            <Plus className="size-4" /> Buat periode
          </Button>
        }
      />
      <Card bodyClassName="">
        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : list.data.length === 0 ? (
          <EmptyState title="Belum ada periode" description="Atur komponen ubar dulu, lalu buat periode bulan ini." />
        ) : (
          <Table actions="always">
            <thead>
              <tr>
                <Th>Periode</Th>
                <Th>Rentang presensi</Th>
                <Th>Status</Th>
                <Th align="right">Pejuang</Th>
                <Th align="right">Total ubar bersih</Th>
                <Th align="right">
                  <span className="sr-only">Buka</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {list.data.map((p) => (
                <tr key={p.id} className="hover:bg-slate-50">
                  <Td className="font-medium whitespace-nowrap text-slate-900">
                    <Link href={`/payroll/periode?id=${p.id}`} className="hover:underline">
                      {periodLabel(p)}
                    </Link>
                  </Td>
                  <Td className="whitespace-nowrap">
                    {dateLong(p.start_date)} sampai {dateLong(p.end_date)}
                  </Td>
                  <Td>
                    <PeriodStatusBadge status={p.status} />
                  </Td>
                  <Td align="right" className="tabular-nums">
                    {p.generated_at ? p.employee_count : "-"}
                  </Td>
                  <Td align="right" className="tabular-nums whitespace-nowrap">
                    {p.generated_at ? rupiah(p.total_net) : "Belum dihitung"}
                  </Td>
                  <Td align="right">
                    <Link
                      href={`/payroll/periode?id=${p.id}`}
                      aria-label={`Buka periode ${periodLabel(p)}`}
                      className="inline-flex size-8 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100"
                    >
                      <ChevronRight className="size-4" />
                    </Link>
                  </Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card>

      <Modal
        open={creating}
        title="Buat periode ubar"
        size="sm"
        onClose={() => setCreating(false)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setCreating(false)}>
              Batal
            </Button>
            <Button onClick={() => create.mutate(form)} loading={create.isPending}>
              Buat
            </Button>
          </>
        }
      >
        <div className="grid grid-cols-2 gap-4">
          <Field label="Bulan" htmlFor="period-month" error={errors.month}>
            <Select id="period-month" value={form.month} onChange={(e) => setForm({ ...form, month: Number(e.target.value) })}>
              {MONTHS.map((m, i) => (
                <option key={m} value={i + 1}>
                  {m}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Tahun" htmlFor="period-year" error={errors.year}>
            <Select id="period-year" value={form.year} onChange={(e) => setForm({ ...form, year: Number(e.target.value) })}>
              {[thisYear - 1, thisYear, thisYear + 1].map((y) => (
                <option key={y} value={y}>
                  {y}
                </option>
              ))}
            </Select>
          </Field>
          <p className="col-span-2 text-xs text-slate-600">
            Rentang presensi mengikuti tanggal cutoff di Komponen Ubar → Setelan periode, dan bersambung dengan periode sebelumnya.
          </p>
        </div>
      </Modal>
    </>
  );
}
