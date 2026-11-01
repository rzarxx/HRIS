"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input, Select } from "@/components/ui/form";
import { Modal } from "@/components/ui/modal";
import { api } from "@/lib/api-client";
import { todayWib } from "@/lib/format";
import { useDivisions, useSave, useSubdivisions } from "@/lib/hooks";
import { employeeSchema, validate, type EmployeeInput } from "@/lib/schemas";
import type { EmployeeListItem, ItemResponse } from "@/lib/types";

type FormState = Omit<EmployeeInput, "base_salary"> & { base_salary: string; division_id: string };

function initial(e: EmployeeListItem | null): FormState {
  if (!e) {
    return {
      nip: "",
      full_name: "",
      email: "",
      phone: "",
      division_id: "",
      subdivision_id: "",
      position: "",
      join_date: todayWib(),
      base_salary: "",
      status: "active",
    };
  }
  return {
    nip: e.nip,
    full_name: e.full_name,
    email: e.email ?? "",
    phone: e.phone ?? "",
    division_id: e.division_id,
    subdivision_id: e.subdivision_id,
    position: e.position ?? "",
    join_date: e.join_date ?? "",
    base_salary: String(e.base_salary),
    status: e.status,
  };
}

export function EmployeeForm({ employee, onClose }: { employee: EmployeeListItem | null; onClose: () => void }) {
  const divisions = useDivisions();
  const subdivisions = useSubdivisions();
  const nextNip = useQuery({
    queryKey: ["next-nip"],
    queryFn: () => api.get<ItemResponse<{ nip: string }>>("/admin/employees/next-nip").then((r) => r.data.nip),
    enabled: !employee,
    staleTime: 0,
  });
  const [form, setForm] = useState<FormState>(() => initial(employee));
  const [errors, setErrors] = useState<Record<string, string>>({});

  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => {
    setForm((f) => ({ ...f, [k]: v }));
    setErrors(({ [k]: _cleared, ...rest }) => rest);
  };

  const save = useSave(
    (v: EmployeeInput) => (employee ? api.put(`/admin/employees/${employee.id}`, v) : api.post("/admin/employees", v)),
    { success: employee ? "Data Pejuang diperbarui." : "Pejuang baru ditambahkan.", onDone: onClose, onFieldErrors: setErrors },
  );

  function submit(e: React.FormEvent) {
    e.preventDefault();
    // division_id hanya untuk menyaring pilihan subdiv, tidak dikirim ke API
    const { division_id, base_salary, ...rest } = form;
    const values: EmployeeInput = { ...rest, base_salary: Number(base_salary) };
    const all = { ...(validate(employeeSchema, values) ?? {}) };
    if (!division_id) all.division_id = "Pilih divisi.";
    if (base_salary === "") all.base_salary = "Gaji pokok wajib diisi.";
    setErrors(all);
    if (Object.keys(all).length === 0) save.mutate(values);
  }

  const subOptions = (subdivisions.data ?? []).filter((s) => s.division_id === form.division_id);

  return (
    <Modal
      open
      size="lg"
      title={employee ? `Ubah data ${employee.full_name}` : "Tambah Pejuang"}
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" form="employee-form" loading={save.isPending}>
            Simpan
          </Button>
        </>
      }
    >
      <form id="employee-form" onSubmit={submit} noValidate className="grid gap-4 sm:grid-cols-2">
        <Field label="Nama lengkap" htmlFor="full_name" error={errors.full_name} required className="sm:col-span-2">
          <Input id="full_name" value={form.full_name} onChange={(e) => set("full_name", e.target.value)} invalid={!!errors.full_name} autoFocus />
        </Field>
        <Field label="NIP" htmlFor="nip" error={errors.nip} hint={employee ? undefined : "Kosongkan untuk NIP otomatis."}>
          <Input
            id="nip"
            value={form.nip ?? ""}
            onChange={(e) => set("nip", e.target.value)}
            invalid={!!errors.nip}
            placeholder={nextNip.data ?? ""}
            className="font-mono"
          />
        </Field>
        <Field label="Jabatan" htmlFor="position" error={errors.position}>
          <Input id="position" value={form.position ?? ""} onChange={(e) => set("position", e.target.value)} placeholder="Staf Lapangan" />
        </Field>
        <Field label="Email" htmlFor="email" error={errors.email}>
          <Input id="email" type="email" value={form.email ?? ""} onChange={(e) => set("email", e.target.value)} invalid={!!errors.email} />
        </Field>
        <Field label="Nomor HP" htmlFor="phone" error={errors.phone}>
          <Input id="phone" inputMode="tel" value={form.phone ?? ""} onChange={(e) => set("phone", e.target.value)} invalid={!!errors.phone} placeholder="081234567890" />
        </Field>
        <Field label="Divisi" htmlFor="division_id" error={errors.division_id} required>
          <Select
            id="division_id"
            value={form.division_id}
            onChange={(e) => setForm((f) => ({ ...f, division_id: e.target.value, subdivision_id: "" }))}
            invalid={!!errors.division_id}
          >
            <option value="">— Pilih divisi —</option>
            {divisions.data?.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Subdiv" htmlFor="subdivision_id" error={errors.subdivision_id} required>
          <Select
            id="subdivision_id"
            value={form.subdivision_id}
            onChange={(e) => set("subdivision_id", e.target.value)}
            invalid={!!errors.subdivision_id}
            disabled={!form.division_id}
          >
            <option value="">{form.division_id ? "— Pilih subdiv —" : "Pilih divisi dulu"}</option>
            {subOptions.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Tanggal bergabung" htmlFor="join_date" error={errors.join_date}>
          <Input id="join_date" type="date" value={form.join_date ?? ""} onChange={(e) => set("join_date", e.target.value)} invalid={!!errors.join_date} />
        </Field>
        <Field label="Gaji pokok (Rp)" htmlFor="base_salary" error={errors.base_salary} required>
          <Input
            id="base_salary"
            inputMode="numeric"
            value={form.base_salary ? Number(form.base_salary.replace(/\D/g, "")).toLocaleString("id-ID") : ""}
            onChange={(e) => set("base_salary", e.target.value.replace(/\D/g, ""))}
            invalid={!!errors.base_salary}
            placeholder="4.500.000"
            className="tabular-nums"
          />
        </Field>
        <Field label="Status" htmlFor="status">
          <Select id="status" value={form.status} onChange={(e) => set("status", e.target.value as FormState["status"])}>
            <option value="active">Aktif</option>
            <option value="inactive">Nonaktif</option>
          </Select>
        </Field>
      </form>
    </Modal>
  );
}
