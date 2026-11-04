"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Toggle } from "@/components/ui/form";
import { EmptyState, ErrorState, Loading } from "@/components/ui/layout";
import { Modal } from "@/components/ui/modal";
import { api } from "@/lib/api-client";
import { useEmployeeLocations, useOfficeLocations, useSave } from "@/lib/hooks";
import type { EmployeeListItem } from "@/lib/types";

export function EmployeeLocationsModal({ employee, onClose }: { employee: EmployeeListItem; onClose: () => void }) {
  const locations = useOfficeLocations();
  const assigned = useEmployeeLocations(employee.id);
  const [selected, setSelected] = useState<string[] | null>(null);

  useEffect(() => {
    if (assigned.data && selected === null) setSelected(assigned.data.map((l) => l.id));
  }, [assigned.data, selected]);

  const save = useSave((ids: string[]) => api.put(`/admin/employees/${employee.id}/locations`, { location_ids: ids }), {
    success: "Lokasi absen tersimpan.",
    onDone: onClose,
  });

  function toggle(id: string) {
    setSelected((cur) => ((cur ?? []).includes(id) ? (cur ?? []).filter((x) => x !== id) : [...(cur ?? []), id]));
  }

  const error = locations.error ?? assigned.error;
  return (
    <Modal
      open
      title={`Lokasi absen ${employee.full_name}`}
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button onClick={() => selected && save.mutate(selected)} loading={save.isPending} disabled={selected === null}>
            Simpan
          </Button>
        </>
      }
    >
      {error ? (
        <ErrorState message={error.message} />
      ) : !locations.data || selected === null ? (
        <Loading />
      ) : locations.data.length === 0 ? (
        <EmptyState title="Belum ada lokasi kantor" description="Tambahkan dulu di menu Lokasi Kantor." />
      ) : (
        <>
          <p className="mb-3 text-sm text-slate-500">Tanpa lokasi yang dipilih, Pejuang ini tidak bisa absen di mana pun.</p>
          <ul className="grid grid-cols-1 gap-2">
            {locations.data.map((l) => (
              <li key={l.id}>
                <Toggle checked={selected.includes(l.id)} onChange={() => toggle(l.id)}>
                  {l.name}
                  {!l.is_active && <span className="ml-1 text-xs text-slate-500">(nonaktif)</span>}
                </Toggle>
              </li>
            ))}
          </ul>
        </>
      )}
    </Modal>
  );
}
