"use client";

import { ArrowLeft, Calculator, CheckCircle2, Download, Search, Send, Trash2, TriangleAlert, Undo2, Wallet } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useMemo, useState } from "react";
import { EmployeePayrollModal } from "@/components/payroll/employee-payroll-modal";
import { PeriodStatusBadge, periodLabel } from "@/components/payroll/status";
import { Button } from "@/components/ui/button";
import { Field, Input, Textarea } from "@/components/ui/form";
import { Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog, Modal } from "@/components/ui/modal";
import { Table, Td, Th } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { api } from "@/lib/api-client";
import { downloadCsv } from "@/lib/csv";
import { dateLong, dateTime, rupiah } from "@/lib/format";
import { useMe, usePayrollPeriod, useSave } from "@/lib/hooks";

type Action = "generate" | "submit" | "finalize" | "mark-paid";

const ACTION_TEXT: Record<Action, string> = {
  generate: "Ubar dihitung ulang.",
  submit: "Periode diajukan untuk disetujui.",
  finalize: "Periode final. Slip ubar terbit.",
  "mark-paid": "Periode ditandai sudah dibayar.",
};

function PeriodDetail() {
  const id = useSearchParams().get("id");
  const router = useRouter();
  const me = useMe();
  const toast = useToast();
  const canFinalizePermission = me.data?.permissions.includes("payroll:finalize") ?? false;
  const detail = usePayrollPeriod(id);
  const [q, setQ] = useState("");
  const [opened, setOpened] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<"finalize" | "delete" | null>(null);
  const [rejecting, setRejecting] = useState(false);
  const [note, setNote] = useState("");
  const [noteError, setNoteError] = useState("");

  const act = useSave((a: Action) => api.post(`/admin/payroll/periods/${id}/${a}`), {
    onDone: () => setConfirm(null),
  });
  const reject = useSave((n: string) => api.post(`/admin/payroll/periods/${id}/reject`, { note: n }), {
    success: "Periode dikembalikan ke draft.",
    onDone: () => {
      setRejecting(false);
      setNote("");
    },
    onFieldErrors: (f) => setNoteError(f.note ?? ""),
  });
  const remove = useSave(() => api.del(`/admin/payroll/periods/${id}`), {
    success: "Periode dihapus.",
    onDone: () => router.push("/payroll"),
  });

  const rows = useMemo(() => {
    const all = detail.data?.rows ?? [];
    const term = q.trim().toLowerCase();
    return term ? all.filter((r) => r.full_name.toLowerCase().includes(term) || r.nip.toLowerCase().includes(term)) : all;
  }, [detail.data, q]);

  if (!id) return <EmptyState title="Periode tidak ditemukan" description="Buka periode dari daftar Periode Ubar." />;
  if (detail.isPending) return <Loading />;
  if (detail.isError) return <ErrorState message={detail.error.message} onRetry={() => detail.refetch()} />;

  const { period, totals, readiness, item_totals: itemTotals } = detail.data;
  const draft = period.status === "draft";
  const generated = !!period.generated_at;
  const notReady = !!readiness && (readiness.review > 0 || readiness.missed_clock_out > 0);
  const run = (a: Action) => act.mutate(a, { onSuccess: () => toast.success(ACTION_TEXT[a]) });

  function exportCsv() {
    const data = detail.data;
    if (!data) return;
    const codes = data.item_totals.filter((t) => t.code !== "UBAR_POKOK");
    downloadCsv(
      `ubar-${period.year}-${String(period.month).padStart(2, "0")}.csv`,
      ["NIPY", "Nama", "Divisi", "Subdiv", "Ubar pokok", ...codes.map((c) => (c.kind === "deduction" ? `Pot. ${c.name}` : c.name)), "Pendapatan", "Potongan", "Ubar bersih"],
      data.rows.map((r) => [
        r.nip,
        r.full_name,
        r.division,
        r.subdivision,
        r.base_salary,
        ...codes.map((c) => c.by_employee[r.employee_id] ?? "0.00"),
        r.gross,
        r.deductions,
        r.net,
      ]),
    );
  }

  return (
    <>
      <Link href="/payroll" className="mb-3 inline-flex items-center gap-1 text-sm text-slate-600 hover:text-slate-900">
        <ArrowLeft className="size-4" /> Periode ubar
      </Link>
      <PageHeader
        title={`Ubar ${periodLabel(period)}`}
        description={`Presensi ${dateLong(period.start_date)} sampai ${dateLong(period.end_date)}.`}
        actions={
          <div className="flex flex-wrap gap-2">
            {generated && (
              <Button variant="secondary" onClick={exportCsv}>
                <Download className="size-4" /> Ekspor CSV
              </Button>
            )}
            {draft && (
              <>
                <Button variant="secondary" onClick={() => setConfirm("delete")} aria-label="Hapus periode">
                  <Trash2 className="size-4" />
                </Button>
                <Button variant={generated ? "secondary" : "primary"} loading={act.isPending && act.variables === "generate"} onClick={() => run("generate")}>
                  <Calculator className="size-4" /> {generated ? "Hitung ulang" : "Hitung ubar"}
                </Button>
                {generated && (
                  <Button onClick={() => run("submit")} loading={act.isPending && act.variables === "submit"} disabled={notReady}>
                    <Send className="size-4" /> Ajukan
                  </Button>
                )}
              </>
            )}
            {period.status === "pending_approval" && canFinalizePermission && (
              <>
                <Button variant="secondary" onClick={() => setRejecting(true)}>
                  <Undo2 className="size-4" /> Tolak
                </Button>
                <Button onClick={() => setConfirm("finalize")} disabled={!period.can_finalize}>
                  <CheckCircle2 className="size-4" /> Setujui & finalkan
                </Button>
              </>
            )}
            {period.status === "final" && canFinalizePermission && (
              <Button onClick={() => run("mark-paid")} loading={act.isPending && act.variables === "mark-paid"}>
                <Wallet className="size-4" /> Tandai sudah dibayar
              </Button>
            )}
          </div>
        }
      />

      <div className="mb-4 flex flex-wrap items-center gap-3 text-sm text-slate-600">
        <PeriodStatusBadge status={period.status} />
        {period.generated_at && <span>Dihitung {dateTime(period.generated_at)}</span>}
        {period.submitted_at && <span>· Diajukan {dateTime(period.submitted_at)}</span>}
        {period.finalized_at && <span>· Final {dateTime(period.finalized_at)}</span>}
        {period.paid_at && <span>· Dibayar {dateTime(period.paid_at)}</span>}
      </div>

      {period.reject_note && draft && (
        <div className="mb-4 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900">
          <strong>Dikembalikan pemeriksa:</strong> {period.reject_note}
        </div>
      )}
      {period.status === "pending_approval" && !period.can_finalize && (
        <div className="mb-4 rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-700">
          Anda yang menghitung atau mengajukan periode ini, jadi persetujuannya dilakukan SDM lain.
        </div>
      )}
      {draft && notReady && readiness && (
        <div className="mb-4 flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900">
          <TriangleAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
          <span>
            Belum bisa diajukan.
            {readiness.review > 0 && (
              <>
                {" "}
                {readiness.review} presensi menunggu review; selesaikan di{" "}
                <Link href="/presensi/review" className="font-medium underline">
                  Review Presensi
                </Link>
                .
              </>
            )}
            {readiness.missed_clock_out > 0 &&
              ` ${readiness.missed_clock_out} presensi belum absen pulang akan ditutup otomatis oleh sistem paling lambat esok pagi.`}{" "}
            Setelah itu, hitung ulang.
          </span>
        </div>
      )}

      {!generated ? (
        <Card>
          <EmptyState title="Ubar belum dihitung" description="Tekan Hitung ubar untuk menghitung dari presensi dan komponen ubar yang aktif." />
        </Card>
      ) : (
        <Card bodyClassName="">
          <div className="space-y-4 border-b border-slate-100 p-4">
            <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
              {[
                ["Total ubar pokok", totals.base_salary],
                ["Total pendapatan", totals.gross],
                ["Total potongan", totals.deductions],
                ["Total ubar bersih", totals.net],
              ].map(([k, v]) => (
                <div key={k} className="rounded-md bg-slate-50 px-4 py-3">
                  <p className="text-xs text-slate-500">{k}</p>
                  <p className="mt-0.5 text-lg font-semibold text-slate-900 tabular-nums">{rupiah(v)}</p>
                </div>
              ))}
            </div>
            {itemTotals.length > 1 && (
              <p className="text-xs text-slate-600">
                {itemTotals
                  .filter((t) => t.code !== "UBAR_POKOK")
                  .map((t) => `${t.name} ${t.kind === "deduction" ? "−" : ""}${rupiah(t.total)}`)
                  .join(" · ")}
              </p>
            )}
            <div className="relative max-w-sm">
              <Search className="pointer-events-none absolute top-2.5 left-3 size-4 text-slate-400" aria-hidden />
              <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Cari nama atau NIPY" aria-label="Cari Pejuang" className="pl-9" />
            </div>
          </div>
          {rows.length === 0 ? (
            <EmptyState title="Tidak ada Pejuang" description="Ubah kata pencarian." />
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>NIPY</Th>
                  <Th>Nama</Th>
                  <Th align="right">Hadir</Th>
                  <Th align="right">Telat</Th>
                  <Th align="right">Alpha</Th>
                  <Th align="right">Ubar pokok</Th>
                  <Th align="right">Pendapatan</Th>
                  <Th align="right">Potongan</Th>
                  <Th align="right">Ubar bersih</Th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={r.employee_id} className="cursor-pointer hover:bg-slate-50" onClick={() => setOpened(r.employee_id)}>
                    <Td className="font-mono text-xs">{r.nip}</Td>
                    <Td className="whitespace-nowrap">
                      <button type="button" className="font-medium text-slate-900 hover:underline" onClick={() => setOpened(r.employee_id)}>
                        {r.full_name}
                      </button>
                      <span className="block text-xs text-slate-500">
                        {r.subdivision}
                        {r.has_inputs && " · ada isian"}
                      </span>
                    </Td>
                    <Td align="right" className="tabular-nums">{r.attendance.present_days}</Td>
                    <Td align="right" className="tabular-nums">{r.attendance.late_count}</Td>
                    <Td align="right" className="tabular-nums">{r.attendance.absent_days}</Td>
                    <Td align="right" className="tabular-nums whitespace-nowrap">{rupiah(r.base_salary)}</Td>
                    <Td align="right" className="tabular-nums whitespace-nowrap">{rupiah(r.gross)}</Td>
                    <Td align="right" className={`tabular-nums whitespace-nowrap ${Number(r.deductions) > 0 ? "text-red-700" : "text-slate-400"}`}>
                      {Number(r.deductions) > 0 ? `−${rupiah(r.deductions)}` : "-"}
                    </Td>
                    <Td align="right" className="font-semibold tabular-nums whitespace-nowrap text-slate-900">
                      {rupiah(r.net)}
                      {r.net_clamped && <span className="block text-xs font-normal text-amber-700">dibulatkan ke 0</span>}
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </Card>
      )}

      {opened && <EmployeePayrollModal periodId={period.id} employeeId={opened} editable={draft} onClose={() => setOpened(null)} />}

      <ConfirmDialog
        open={confirm === "finalize"}
        title="Setujui dan finalkan?"
        confirmLabel="Finalkan"
        message={
          <>
            Ubar {periodLabel(period)} untuk {period.employee_count} Pejuang, total <strong className="text-slate-800">{rupiah(totals.net)}</strong>, akan final
            dan slip terbit. Periode final tidak bisa dibuka lagi.
          </>
        }
        loading={act.isPending}
        onConfirm={() => run("finalize")}
        onClose={() => setConfirm(null)}
      />
      <ConfirmDialog
        open={confirm === "delete"}
        title="Hapus periode?"
        message={<>Periode {periodLabel(period)} beserta hasil hitung dan isiannya akan dihapus.</>}
        loading={remove.isPending}
        onConfirm={() => remove.mutate(undefined)}
        onClose={() => setConfirm(null)}
      />
      <Modal
        open={rejecting}
        title="Tolak periode"
        size="sm"
        onClose={() => setRejecting(false)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setRejecting(false)}>
              Batal
            </Button>
            <Button variant="danger" onClick={() => reject.mutate(note)} loading={reject.isPending}>
              Tolak
            </Button>
          </>
        }
      >
        <Field label="Alasan" htmlFor="reject-note" error={noteError} required hint="Dibaca SDM yang menghitung, lalu periode kembali ke draft.">
          <Textarea id="reject-note" rows={3} value={note} onChange={(e) => setNote(e.target.value)} invalid={!!noteError} />
        </Field>
      </Modal>
    </>
  );
}

export default function PayrollPeriodPage() {
  return (
    <Suspense fallback={<Loading />}>
      <PeriodDetail />
    </Suspense>
  );
}
