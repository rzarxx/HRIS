"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/form";
import { Badge, EmptyState, ErrorState, Loading } from "@/components/ui/layout";
import { Modal } from "@/components/ui/modal";
import { api } from "@/lib/api-client";
import { useEmployeePayrollComponents, useSave } from "@/lib/hooks";
import type { EmployeeListItem } from "@/lib/types";

/** Nilai komponen bercara hitung "Nominal per Pejuang" (D66). Kosong berarti tidak berlaku untuk Pejuang ini. */
export function EmployeePayrollModal({ employee, onClose }: { employee: EmployeeListItem; onClose: () => void }) {
  const list = useEmployeePayrollComponents(employee.id);
  const [values, setValues] = useState<Record<string, string> | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (list.data && values === null) {
      setValues(Object.fromEntries(list.data.map((c) => [c.component_id, c.amount ? String(Number(c.amount)) : ""])));
    }
  }, [list.data, values]);

  const save = useSave(
    (v: Record<string, string>) =>
      api.put(`/admin/employees/${employee.id}/payroll-components`, {
        components: Object.entries(v)
          .filter(([, amount]) => amount !== "")
          .map(([component_id, amount]) => ({ component_id, amount })),
      }),
    { success: "Komponen ubar Pejuang tersimpan.", onDone: onClose, onFieldErrors: (f) => setError(Object.values(f).join(" ")) },
  );

  return (
    <Modal
      open
      title={`Komponen ubar ${employee.full_name}`}
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button onClick={() => values && save.mutate(values)} loading={save.isPending} disabled={values === null || !list.data?.length}>
            Simpan
          </Button>
        </>
      }
    >
      {list.isError ? (
        <ErrorState message={list.error.message} />
      ) : !list.data || values === null ? (
        <Loading />
      ) : list.data.length === 0 ? (
        <EmptyState
          title="Belum ada komponen per Pejuang"
          description="Buat komponen dengan cara hitung Nominal per Pejuang lebih dulu."
          action={
            <Link href="/payroll/komponen" className="text-sm font-medium text-primary-700 underline">
              Buka Komponen Ubar
            </Link>
          }
        />
      ) : (
        <>
          <p className="mb-3 text-sm text-slate-600">Kosongkan nominal bila komponen tidak berlaku untuk Pejuang ini.</p>
          <ul className="grid grid-cols-1 gap-3">
            {list.data.map((c) => (
              <li key={c.component_id} className="grid grid-cols-1 items-center gap-2 sm:grid-cols-[1fr_12rem]">
                <label htmlFor={`pc-${c.component_id}`} className="text-sm text-slate-800">
                  <span className="font-medium">{c.name}</span>{" "}
                  {c.kind === "earning" ? <Badge tone="green">Pendapatan</Badge> : <Badge tone="red">Potongan</Badge>}
                  {!c.active && <span className="ml-1 text-xs text-slate-500">(nonaktif)</span>}
                </label>
                <Input
                  id={`pc-${c.component_id}`}
                  inputMode="numeric"
                  placeholder="Rp"
                  value={values[c.component_id] ? Number(values[c.component_id]).toLocaleString("id-ID") : ""}
                  onChange={(e) => setValues({ ...values, [c.component_id]: e.target.value.replace(/\D/g, "") })}
                />
              </li>
            ))}
          </ul>
          {error && <p className="mt-3 text-sm text-red-600">{error}</p>}
        </>
      )}
    </Modal>
  );
}
