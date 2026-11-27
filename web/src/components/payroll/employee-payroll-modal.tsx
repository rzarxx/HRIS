"use client";

import { Download } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/form";
import { ErrorState, Loading } from "@/components/ui/layout";
import { Modal } from "@/components/ui/modal";
import { api } from "@/lib/api-client";
import { downloadBlob } from "@/lib/download";
import { rupiah } from "@/lib/format";
import { useEmployeePayroll, useSave } from "@/lib/hooks";

type Draft = Record<string, { value: string; note: string }>;

const trim = (v: string | null) => (v == null ? "" : String(Number(v)));

/** Rincian slip satu Pejuang dan isian per periode (lembur, potongan manual). Isian hanya bisa diubah saat draft. */
export function EmployeePayrollModal({
  periodId,
  employeeId,
  editable,
  onClose,
}: {
  periodId: string;
  employeeId: string;
  editable: boolean;
  onClose: () => void;
}) {
  const detail = useEmployeePayroll(periodId, employeeId);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [downloading, setDownloading] = useState(false);

  // PDF draf diberi tanda DRAF oleh server sampai periode final.
  async function downloadPdf() {
    if (!detail.data) return;
    setDownloading(true);
    try {
      const p = detail.data.period;
      const blob = await api.blob(`/admin/payroll/periods/${periodId}/employees/${employeeId}/pdf`);
      downloadBlob(blob, `Slip-Ubar-${p.year}-${String(p.month).padStart(2, "0")}-${detail.data.row.nip}.pdf`);
    } finally {
      setDownloading(false);
    }
  }

  useEffect(() => {
    if (detail.data && draft === null) {
      setDraft(
        Object.fromEntries(
          detail.data.inputs.map((i) => [i.component_id, { value: trim(i.method === "per_unit" ? i.quantity : i.amount), note: i.note }]),
        ),
      );
    }
  }, [detail.data, draft]);

  const save = useSave(
    (d: Draft) =>
      api.put(`/admin/payroll/periods/${periodId}/employees/${employeeId}`, {
        inputs: (detail.data?.inputs ?? [])
          .filter((i) => d[i.component_id]?.value)
          .map((i) => ({
            component_id: i.component_id,
            note: d[i.component_id].note,
            ...(i.method === "per_unit" ? { quantity: d[i.component_id].value } : { amount: d[i.component_id].value }),
          })),
      }),
    { success: "Isian tersimpan dan ubar dihitung ulang.", onDone: () => setDraft(null), onFieldErrors: setErrors },
  );

  const data = detail.data;
  const att = data?.row.attendance;
  return (
    <Modal
      open
      size="lg"
      title={data ? `Rincian ubar ${data.row.full_name}` : "Rincian ubar"}
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Tutup
          </Button>
          {data && (
            <Button variant="secondary" onClick={downloadPdf} loading={downloading}>
              <Download className="size-4" /> {data.period.status === "final" || data.period.status === "paid" ? "Unduh slip PDF" : "Pratinjau PDF (draf)"}
            </Button>
          )}
          {editable && !!data?.inputs.length && (
            <Button onClick={() => draft && save.mutate(draft)} loading={save.isPending} disabled={!draft}>
              Simpan isian
            </Button>
          )}
        </>
      }
    >
      {detail.isError ? (
        <ErrorState message={detail.error.message} />
      ) : !data || !att ? (
        <Loading />
      ) : (
        <div className="space-y-5">
          <p className="text-sm text-slate-600">
            {data.row.nip} · {data.row.subdivision}, {data.row.division}
          </p>
          <dl className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
            {[
              ["Hari kerja", Number(att.work_days).toLocaleString("id-ID")],
              ["Hadir", att.present_days],
              ["Telat", `${att.late_count} kali, ${att.late_minutes} menit`],
              ["Alpha", att.absent_days],
              ["Cuti/izin", att.leave_days],
              ["Cuti tidak dibayar", att.unpaid_leave_days],
            ].map(([k, v]) => (
              <div key={k} className="rounded-md bg-slate-50 px-3 py-2">
                <dt className="text-xs text-slate-500">{k}</dt>
                <dd className="font-medium text-slate-900 tabular-nums">{v}</dd>
              </div>
            ))}
          </dl>

          <table className="w-full text-sm">
            <tbody>
              {data.items.map((it) => (
                <tr key={it.code} className="border-b border-slate-100">
                  <td className="py-2 pr-2 text-slate-800">
                    {it.name}
                    {it.quantity && it.rate && (
                      <span className="ml-1 text-xs text-slate-500">
                        ({Number(it.quantity).toLocaleString("id-ID")} × {rupiah(it.rate)})
                      </span>
                    )}
                    {!it.show_on_slip && <span className="ml-1 text-xs text-slate-500">(tidak tampil di slip)</span>}
                  </td>
                  <td className={`py-2 text-right tabular-nums whitespace-nowrap ${it.kind === "deduction" ? "text-red-700" : "text-slate-900"}`}>
                    {it.kind === "deduction" ? "−" : ""}
                    {rupiah(it.amount)}
                  </td>
                </tr>
              ))}
              <tr>
                <td className="pt-3 font-semibold text-slate-900">Ubar bersih</td>
                <td className="pt-3 text-right font-semibold tabular-nums text-slate-900">{rupiah(data.row.net)}</td>
              </tr>
            </tbody>
          </table>
          {data.row.net_clamped && (
            <p className="rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-800">Potongan melebihi pendapatan, ubar bersih dibulatkan ke Rp 0.</p>
          )}

          {data.inputs.length > 0 && draft && (
            <section>
              <h3 className="mb-2 text-sm font-semibold text-slate-900">Isian periode ini</h3>
              <ul className="space-y-3">
                {data.inputs.map((i) => (
                  <li key={i.component_id} className="grid grid-cols-1 gap-2 sm:grid-cols-[1fr_10rem_1fr] sm:items-center">
                    <label htmlFor={`in-${i.component_id}`} className="text-sm text-slate-800">
                      {i.name}
                      <span className="block text-xs text-slate-500">
                        {i.method === "per_unit" ? `Jumlah × ${rupiah(i.rate ?? 0)}` : "Nominal (Rp)"}
                      </span>
                    </label>
                    <Input
                      id={`in-${i.component_id}`}
                      inputMode="decimal"
                      disabled={!editable}
                      value={draft[i.component_id]?.value ?? ""}
                      invalid={!!errors[i.code]}
                      onChange={(e) => setDraft({ ...draft, [i.component_id]: { ...draft[i.component_id], value: e.target.value.replace(/[^\d.]/g, "") } })}
                    />
                    <Input
                      aria-label={`Catatan ${i.name}`}
                      placeholder="Catatan (opsional)"
                      disabled={!editable}
                      value={draft[i.component_id]?.note ?? ""}
                      onChange={(e) => setDraft({ ...draft, [i.component_id]: { ...draft[i.component_id], note: e.target.value } })}
                    />
                    {errors[i.code] && <p className="text-xs text-red-600 sm:col-span-3">{errors[i.code]}</p>}
                  </li>
                ))}
              </ul>
              {errors.inputs && <p className="mt-2 text-xs text-red-600">{errors.inputs}</p>}
              {!editable && <p className="mt-2 text-xs text-slate-500">Isian hanya bisa diubah saat periode masih draft.</p>}
            </section>
          )}
        </div>
      )}
    </Modal>
  );
}
