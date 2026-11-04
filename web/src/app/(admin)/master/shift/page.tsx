"use client";

import { Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { Button, IconButton } from "@/components/ui/button";
import { Field, Input, Select, Toggle } from "@/components/ui/form";
import { Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog, Modal } from "@/components/ui/modal";
import { Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { DAYS, dateLong, todayWib } from "@/lib/format";
import { useHolidays, useSave, useShifts } from "@/lib/hooks";
import { holidaySchema, shiftSchema, validate, type HolidayInput, type ShiftForm } from "@/lib/schemas";
import type { Holiday, Shift } from "@/lib/types";

const EMPTY_SHIFT: ShiftForm = { name: "", start_time: "08:00", end_time: "17:00", late_tolerance_minutes: "10", work_days: [1, 2, 3, 4, 5] };

function workDaysLabel(days: number[]) {
  return days.map((d) => DAYS[d - 1]).join(", ");
}

function ShiftsCard() {
  const list = useShifts();
  const [editing, setEditing] = useState<Shift | "new" | null>(null);
  const [form, setForm] = useState<ShiftForm>(EMPTY_SHIFT);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [deleting, setDeleting] = useState<Shift | null>(null);

  const save = useSave(
    (v: ShiftForm) => {
      const body = { ...v, late_tolerance_minutes: Number(v.late_tolerance_minutes) };
      return editing && editing !== "new" ? api.patch(`/admin/shifts/${editing.id}`, body) : api.post("/admin/shifts", body);
    },
    { success: "Shift tersimpan.", onDone: () => setEditing(null), onFieldErrors: setErrors },
  );
  const remove = useSave((id: string) => api.del(`/admin/shifts/${id}`), {
    success: "Shift dihapus.",
    onDone: () => setDeleting(null),
  });

  function open(s: Shift | "new") {
    setErrors({});
    setForm(
      s === "new"
        ? EMPTY_SHIFT
        : { name: s.name, start_time: s.start_time, end_time: s.end_time, late_tolerance_minutes: String(s.late_tolerance_minutes), work_days: s.work_days },
    );
    setEditing(s);
  }

  function toggleDay(day: number) {
    const days = form.work_days.includes(day) ? form.work_days.filter((d) => d !== day) : [...form.work_days, day].sort((a, b) => a - b);
    setForm({ ...form, work_days: days });
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const fieldErrors = validate(shiftSchema, form);
    setErrors(fieldErrors ?? {});
    if (!fieldErrors) save.mutate(form);
  }

  return (
    <>
      <Card
        title="Shift kerja"
        bodyClassName=""
        actions={
          <Button onClick={() => open("new")}>
            <Plus className="size-4" /> Tambah shift
          </Button>
        }
      >
        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : list.data.length === 0 ? (
          <EmptyState title="Belum ada shift" description="Tambahkan shift pertama." />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>Nama shift</Th>
                <Th>Jam kerja (WIB)</Th>
                <Th>Toleransi telat</Th>
                <Th>Hari kerja</Th>
                <Th align="right">
                  <span className="sr-only">Aksi</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {list.data.map((s) => (
                <tr key={s.id} className="hover:bg-slate-50">
                  <Td className="font-medium text-slate-900">{s.name}</Td>
                  <Td className="whitespace-nowrap">
                    {s.start_time} sampai {s.end_time}
                    {s.end_time < s.start_time && <span className="text-slate-500"> (hari berikutnya)</span>}
                  </Td>
                  <Td>{s.late_tolerance_minutes} menit</Td>
                  <Td>{workDaysLabel(s.work_days)}</Td>
                  <Td align="right" className="whitespace-nowrap">
                    <IconButton label={`Ubah ${s.name}`} onClick={() => open(s)}>
                      <Pencil className="size-4" />
                    </IconButton>
                    <IconButton label={`Hapus ${s.name}`} onClick={() => setDeleting(s)} className="hover:text-red-600">
                      <Trash2 className="size-4" />
                    </IconButton>
                  </Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card>

      <Modal
        open={editing !== null}
        title={editing === "new" ? "Tambah shift" : "Ubah shift"}
        onClose={() => setEditing(null)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setEditing(null)}>
              Batal
            </Button>
            <Button type="submit" form="shift-form" loading={save.isPending}>
              Simpan
            </Button>
          </>
        }
      >
        <form id="shift-form" onSubmit={submit} noValidate className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <Field label="Nama shift" htmlFor="shift-name" error={errors.name} required className="sm:col-span-3">
            <Input id="shift-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} invalid={!!errors.name} placeholder="Reguler" />
          </Field>
          <Field label="Jam masuk" htmlFor="start" error={errors.start_time} required>
            <Input id="start" type="time" value={form.start_time} onChange={(e) => setForm({ ...form, start_time: e.target.value })} invalid={!!errors.start_time} />
          </Field>
          <Field label="Jam pulang" htmlFor="end" error={errors.end_time} required>
            <Input id="end" type="time" value={form.end_time} onChange={(e) => setForm({ ...form, end_time: e.target.value })} invalid={!!errors.end_time} />
          </Field>
          <Field label="Toleransi telat (menit)" htmlFor="tolerance" error={errors.late_tolerance_minutes} required>
            <Input id="tolerance" inputMode="numeric" value={form.late_tolerance_minutes} onChange={(e) => setForm({ ...form, late_tolerance_minutes: e.target.value })} invalid={!!errors.late_tolerance_minutes} />
          </Field>
          <fieldset className="sm:col-span-3">
            <legend className="mb-1 block text-sm font-medium text-slate-700">
              Hari kerja <span className="text-red-600">*</span>
            </legend>
            <div className="flex flex-wrap gap-x-4 gap-y-2">
              {DAYS.map((label, i) => (
                <Toggle key={label} checked={form.work_days.includes(i + 1)} onChange={() => toggleDay(i + 1)}>
                  {label}
                </Toggle>
              ))}
            </div>
            {errors.work_days && <p className="mt-1 text-xs text-red-600">{errors.work_days}</p>}
          </fieldset>
        </form>
      </Modal>

      <ConfirmDialog
        open={deleting !== null}
        title="Hapus shift?"
        message={
          <>
            Shift <strong className="text-slate-800">{deleting?.name}</strong> akan dihapus permanen. Shift yang sudah dipakai Pejuang atau presensi tidak dapat dihapus.
          </>
        }
        loading={remove.isPending}
        onConfirm={() => deleting && remove.mutate(deleting.id)}
        onClose={() => setDeleting(null)}
      />
    </>
  );
}

function HolidaysCard() {
  const thisYear = Number(todayWib().slice(0, 4));
  const [year, setYear] = useState(thisYear);
  const list = useHolidays(year);
  const [adding, setAdding] = useState(false);
  const [form, setForm] = useState<HolidayInput>({ date: "", name: "" });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [deleting, setDeleting] = useState<Holiday | null>(null);

  const save = useSave((v: HolidayInput) => api.post("/admin/holidays", v), {
    success: "Hari libur tersimpan.",
    onDone: () => setAdding(false),
    onFieldErrors: setErrors,
  });
  const remove = useSave((date: string) => api.del(`/admin/holidays/${date}`), {
    success: "Hari libur dihapus.",
    onDone: () => setDeleting(null),
  });

  function open() {
    setErrors({});
    setForm({ date: "", name: "" });
    setAdding(true);
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const fieldErrors = validate(holidaySchema, form);
    setErrors(fieldErrors ?? {});
    if (!fieldErrors) save.mutate(form);
  }

  return (
    <>
      <Card
        title="Hari libur"
        bodyClassName=""
        actions={
          <div className="flex items-center gap-2">
            <Select value={year} onChange={(e) => setYear(Number(e.target.value))} aria-label="Tahun" className="w-auto">
              {[thisYear - 1, thisYear, thisYear + 1].map((y) => (
                <option key={y} value={y}>
                  {y}
                </option>
              ))}
            </Select>
            <Button onClick={open}>
              <Plus className="size-4" /> Tambah
            </Button>
          </div>
        }
      >
        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : list.data.length === 0 ? (
          <EmptyState title={`Belum ada hari libur ${year}`} description="Tambahkan hari libur nasional atau cuti bersama." />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>Tanggal</Th>
                <Th>Keterangan</Th>
                <Th align="right">
                  <span className="sr-only">Aksi</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {list.data.map((h) => (
                <tr key={h.date} className="hover:bg-slate-50">
                  <Td className="whitespace-nowrap">{dateLong(h.date)}</Td>
                  <Td className="font-medium text-slate-900">{h.name}</Td>
                  <Td align="right">
                    <IconButton label={`Hapus ${h.name}`} onClick={() => setDeleting(h)} className="hover:text-red-600">
                      <Trash2 className="size-4" />
                    </IconButton>
                  </Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card>

      <Modal
        open={adding}
        title="Tambah hari libur"
        size="sm"
        onClose={() => setAdding(false)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setAdding(false)}>
              Batal
            </Button>
            <Button type="submit" form="holiday-form" loading={save.isPending}>
              Simpan
            </Button>
          </>
        }
      >
        <form id="holiday-form" onSubmit={submit} noValidate className="grid grid-cols-1 gap-4">
          <Field label="Tanggal" htmlFor="holiday-date" error={errors.date} required>
            <Input id="holiday-date" type="date" value={form.date} onChange={(e) => setForm({ ...form, date: e.target.value })} invalid={!!errors.date} />
          </Field>
          <Field label="Keterangan" htmlFor="holiday-name" error={errors.name} required>
            <Input id="holiday-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} invalid={!!errors.name} placeholder="Hari Kemerdekaan" />
          </Field>
        </form>
      </Modal>

      <ConfirmDialog
        open={deleting !== null}
        title="Hapus hari libur?"
        message={
          <>
            <strong className="text-slate-800">{deleting?.name}</strong> akan dihapus dari kalender.
          </>
        }
        loading={remove.isPending}
        onConfirm={() => deleting && remove.mutate(deleting.date)}
        onClose={() => setDeleting(null)}
      />
    </>
  );
}

export default function ShiftPage() {
  return (
    <>
      <PageHeader title="Shift" description="Jam kerja, toleransi telat, dan hari libur yang dipakai untuk menghitung presensi." />
      <div className="grid grid-cols-1 gap-6">
        <ShiftsCard />
        <HolidaysCard />
      </div>
    </>
  );
}
