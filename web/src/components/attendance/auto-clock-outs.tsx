"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input, Select, Textarea } from "@/components/ui/form";
import { Card, EmptyState, ErrorState } from "@/components/ui/layout";
import { Modal } from "@/components/ui/modal";
import { Pagination, Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { dateLong, dateTime } from "@/lib/format";
import { useAutoClockOuts, useMe, useSave } from "@/lib/hooks";
import type { AttendanceAdjustInput, AutoClockOutItem } from "@/lib/types";

const PER_PAGE = 20;

const ACTION_LABEL: Record<AttendanceAdjustInput["action"], string> = {
  confirm: "Sesuai, jam pulang otomatis dipakai",
  set_time: "Ubah jam pulang",
  absent: "Tidak hadir",
};

/** Jam HH:MM dalam WIB dari waktu ISO, untuk nilai awal isian jam pulang. */
function wibClock(iso: string | null) {
  if (!iso) return "";
  return new Date(iso).toLocaleTimeString("en-GB", { timeZone: "Asia/Jakarta", hour: "2-digit", minute: "2-digit", hour12: false });
}

function AdjustModal({ item, onClose }: { item: AutoClockOutItem; onClose: () => void }) {
  const me = useMe();
  // Koreksi memengaruhi ubar, jadi presensi sendiri dicek SDM lain (server menolak CONFLICT_SELF_ACTION).
  const isSelf = me.data?.employee_id === item.employee_id;
  const [action, setAction] = useState<AttendanceAdjustInput["action"]>("confirm");
  const [time, setTime] = useState(wibClock(item.clock_out_at));
  const [note, setNote] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});

  const save = useSave((v: AttendanceAdjustInput) => api.post(`/admin/attendance/${item.id}/adjust`, v), {
    success: "Presensi tersimpan.",
    onDone: onClose,
    onFieldErrors: setErrors,
  });

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const text = note.trim();
    if (action === "absent" && text.length < 3) {
      setErrors({ note: "Mohon isi catatan minimal 3 karakter bila Pejuang dinyatakan tidak hadir." });
      return;
    }
    if (action === "set_time" && !/^\d{2}:\d{2}$/.test(time)) {
      setErrors({ clock_out_time: "Mohon isi jam pulang." });
      return;
    }
    setErrors({});
    save.mutate({ action, clock_out_time: action === "set_time" ? time : undefined, note: text });
  }

  return (
    <Modal
      open
      title={`Cek pulang otomatis ${item.employee_name}`}
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Batal
          </Button>
          {!isSelf && (
            <Button type="submit" form="adjust-form" loading={save.isPending}>
              Simpan
            </Button>
          )}
        </>
      }
    >
      <dl className="grid grid-cols-3 gap-y-2 text-sm">
        <dt className="text-slate-500">Pejuang</dt>
        <dd className="col-span-2 break-words text-slate-800">
          {item.employee_name} · {item.nip}
        </dd>
        <dt className="text-slate-500">Tanggal kerja</dt>
        <dd className="col-span-2 text-slate-800">{dateLong(item.work_date)}</dd>
        <dt className="text-slate-500">Shift</dt>
        <dd className="col-span-2 text-slate-800">{item.shift_name || "Tanpa shift"}</dd>
        <dt className="text-slate-500">Masuk</dt>
        <dd className="col-span-2 text-slate-800">{dateTime(item.clock_in_at)}</dd>
        <dt className="text-slate-500">Pulang otomatis</dt>
        <dd className="col-span-2 text-slate-800">{dateTime(item.clock_out_at)}</dd>
      </dl>
      {isSelf ? (
        <p className="mt-4 text-sm text-slate-500">Presensi Anda sendiri harus dicek SDM lain.</p>
      ) : (
        <form id="adjust-form" onSubmit={submit} noValidate className="mt-4 grid grid-cols-1 gap-4 border-t border-slate-100 pt-4">
          <Field label="Keputusan" htmlFor="adjust_action" error={errors.action} required>
            <Select id="adjust_action" value={action} onChange={(e) => setAction(e.target.value as AttendanceAdjustInput["action"])} invalid={!!errors.action}>
              {(Object.keys(ACTION_LABEL) as AttendanceAdjustInput["action"][]).map((a) => (
                <option key={a} value={a}>
                  {ACTION_LABEL[a]}
                </option>
              ))}
            </Select>
          </Field>
          {action === "set_time" && (
            <Field label="Jam pulang (WIB)" htmlFor="adjust_time" error={errors.clock_out_time} required hint="Jam sebelum jam masuk dihitung esok hari (shift malam).">
              <Input id="adjust_time" type="time" value={time} onChange={(e) => setTime(e.target.value)} invalid={!!errors.clock_out_time} />
            </Field>
          )}
          <Field label="Catatan" htmlFor="adjust_note" error={errors.note} required={action === "absent"} hint={action === "absent" ? undefined : "Opsional."}>
            <Textarea id="adjust_note" rows={2} maxLength={500} value={note} onChange={(e) => setNote(e.target.value)} invalid={!!errors.note} />
          </Field>
        </form>
      )}
    </Modal>
  );
}

/** Presensi lupa absen pulang yang ditutup sistem dengan jam akhir shift, menunggu dicek SDM (D69). */
export function AutoClockOuts() {
  const [page, setPage] = useState(1);
  const list = useAutoClockOuts({ page, per_page: PER_PAGE });
  const [checking, setChecking] = useState<AutoClockOutItem | null>(null);

  if (list.isPending) return null;
  if (list.isError) return <ErrorState message={list.error.message} onRetry={() => list.refetch()} />;
  if (list.data.meta.total === 0 && page === 1) return null;

  return (
    <>
      <Card title="Pulang otomatis" className="mb-6" bodyClassName="">
        <p className="border-b border-slate-100 px-4 py-3 text-sm text-slate-600">
          Pejuang lupa absen pulang. Sistem memakai jam akhir shift; cek dan ubah bila perlu.
        </p>
        {list.data.data.length === 0 ? (
          <EmptyState title="Semua sudah dicek" />
        ) : (
          <>
            <Table actions="always">
              <thead>
                <tr>
                  <Th>Pejuang</Th>
                  <Th>Tanggal kerja</Th>
                  <Th>Masuk</Th>
                  <Th>Pulang otomatis</Th>
                  <Th align="right">
                    <span className="sr-only">Aksi</span>
                  </Th>
                </tr>
              </thead>
              <tbody className={list.isPlaceholderData ? "opacity-60" : ""}>
                {list.data.data.map((a) => (
                  <tr key={a.id} className="hover:bg-slate-50">
                    <Td>
                      <p className="font-medium text-slate-900">{a.employee_name}</p>
                      <p className="text-xs text-slate-500">
                        {a.nip} · {a.subdivision_name}
                      </p>
                    </Td>
                    <Td className="whitespace-nowrap">{dateLong(a.work_date)}</Td>
                    <Td className="whitespace-nowrap">{wibClock(a.clock_in_at)}</Td>
                    <Td className="whitespace-nowrap">{wibClock(a.clock_out_at)}</Td>
                    <Td align="right">
                      <Button variant="secondary" size="sm" onClick={() => setChecking(a)} aria-label={`Cek pulang otomatis ${a.employee_name}, ${dateLong(a.work_date)}`}>
                        Cek
                      </Button>
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
            <Pagination page={page} perPage={PER_PAGE} total={list.data.meta.total} onPage={setPage} />
          </>
        )}
      </Card>
      {checking && <AdjustModal item={checking} onClose={() => setChecking(null)} />}
    </>
  );
}

