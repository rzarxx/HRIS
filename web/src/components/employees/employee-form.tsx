"use client";

import { Plus, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { Button, IconButton } from "@/components/ui/button";
import { Field, Input, Select } from "@/components/ui/form";
import { Loading } from "@/components/ui/layout";
import { Modal } from "@/components/ui/modal";
import { api } from "@/lib/api-client";
import { todayWib } from "@/lib/format";
import { useBranches, useDivisions, useEmployee, useMe, useSave, useShifts, useSubdivisions } from "@/lib/hooks";
import {
  CHILD_RELATIONS,
  EDUCATION_LEVELS,
  employeeSchema,
  GENDERS,
  MARITAL_STATUSES,
  validate,
  type ChildInput,
  type EmployeeInput,
} from "@/lib/schemas";
import type { EmployeeCreated, EmployeeListItem, ItemResponse } from "@/lib/types";

const MAX_CHILDREN = 10;

type FormState = Omit<EmployeeInput, "base_salary" | "children_count" | "children"> & {
  base_salary: string;
  branch_id: string;
  division_id: string;
  children_count: string;
};

function initial(e: EmployeeListItem | null): FormState {
  if (!e) {
    return {
      nip: "",
      full_name: "",
      email: "",
      phone: "",
      gender: null,
      marital_status: null,
      has_children: null,
      children_count: "",
      branch_id: "",
      division_id: "",
      subdivision_id: "",
      position: "",
      join_date: todayWib(),
      base_salary: "",
      status: "active",
      default_shift_id: null,
    };
  }
  return {
    nip: e.nip,
    full_name: e.full_name,
    email: e.email ?? "",
    phone: e.phone ?? "",
    gender: e.gender,
    marital_status: e.marital_status,
    has_children: e.has_children ?? null,
    children_count: e.children_count == null ? "" : String(e.children_count),
    branch_id: e.branch_id,
    division_id: e.division_id,
    subdivision_id: e.subdivision_id,
    position: e.position ?? "",
    join_date: e.join_date ?? "",
    base_salary: (e.base_salary ?? "0").split(".")[0],
    status: e.status,
    default_shift_id: e.default_shift_id,
  };
}

const blank = (v: string | null | undefined) => (v && v.trim() ? v.trim() : null);

function Section({ title, description, children }: { title: string; description?: string; children: React.ReactNode }) {
  return (
    <fieldset className="col-span-full grid grid-cols-1 gap-4 border-t border-slate-100 pt-4 first:border-t-0 first:pt-0 sm:grid-cols-2">
      <legend className="col-span-full mb-1 text-sm font-semibold text-slate-800">
        {title}
        {description && <span className="mt-0.5 block text-xs font-normal text-slate-500">{description}</span>}
      </legend>
      {children}
    </fieldset>
  );
}

export function EmployeeForm({
  employee,
  onClose,
  onCreated,
}: {
  employee: EmployeeListItem | null;
  onClose: () => void;
  onCreated: (created: EmployeeCreated) => void;
}) {
  const branches = useBranches();
  const divisions = useDivisions();
  const subdivisions = useSubdivisions();
  const shifts = useShifts();
  const me = useMe();
  // Daftar Pejuang tidak memuat data anak, jadi detail dimuat saat mengubah.
  const detail = useEmployee(employee?.id ?? null);
  // Ubar hanya urusan SDM; super admin tidak melihat atau mengubahnya (D34).
  const managesSalary = me.data?.role === "sdm";
  const multiBranch = (branches.data?.length ?? 0) > 1;
  const [form, setForm] = useState<FormState>(() => initial(employee));
  const [children, setChildren] = useState<ChildInput[] | null>(employee ? null : []);
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (children === null && detail.data) {
      setChildren(
        (detail.data.children ?? []).map((c) => ({
          ...c,
          birth_place: c.birth_place ?? "",
          birth_date: c.birth_date ?? "",
          school_name: c.school_name ?? "",
        })),
      );
    }
  }, [children, detail.data]);

  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => {
    setForm((f) => ({ ...f, [k]: v }));
    setErrors(({ [k]: _cleared, ...rest }) => rest);
  };

  const setChild = (i: number, patch: Partial<ChildInput>) => {
    setChildren((list) => (list ?? []).map((c, j) => (j === i ? { ...c, ...patch } : c)));
    setErrors((errs) => Object.fromEntries(Object.entries(errs).filter(([k]) => !k.startsWith(`children.${i}.`))));
  };

  function addChild() {
    setChildren((list) => {
      const current = list ?? [];
      const next = Math.max(0, ...current.map((c) => c.child_order)) + 1;
      return [...current, { child_order: next, full_name: "", gender: null, birth_place: "", birth_date: "", relation: null, education: null, school_name: "" }];
    });
    setForm((f) => ({ ...f, has_children: true }));
  }

  const save = useSave(
    (v: EmployeeInput) =>
      employee ? api.patch<ItemResponse<EmployeeListItem>>(`/admin/employees/${employee.id}`, v) : api.post<ItemResponse<EmployeeCreated>>("/admin/employees", v),
    {
      success: employee ? "Data Pejuang diperbarui." : "Pejuang baru ditambahkan.",
      onDone: (res) => {
        onClose();
        if (!employee) onCreated(res.data as EmployeeCreated);
      },
      onFieldErrors: setErrors,
    },
  );

  function submit(e: React.FormEvent) {
    e.preventDefault();
    // branch_id dan division_id hanya menyaring pilihan, tidak dikirim ke API
    const { branch_id: _branch, division_id, base_salary, children_count, ...rest } = form;
    const count = children_count.trim() === "" ? null : Number(children_count);
    const values: EmployeeInput = {
      ...rest,
      email: blank(rest.email),
      phone: blank(rest.phone),
      children_count: count,
      // Data anak hanya dikirim setelah termuat, agar tidak menghapus data lama.
      ...(children !== null && {
        children: children.map((c) => ({
          ...c,
          full_name: c.full_name.trim(),
          birth_place: blank(c.birth_place),
          birth_date: blank(c.birth_date),
          school_name: blank(c.school_name),
        })),
      }),
      ...(managesSalary && { base_salary: base_salary ? `${base_salary}.00` : "" }),
    };
    const all = { ...(validate(employeeSchema, values) ?? {}) };
    if (!division_id) all.division_id = "Pilih divisi.";
    if (managesSalary && base_salary === "") all.base_salary = "Ubar pokok wajib diisi.";
    if (count !== null && Number.isNaN(count)) all.children_count = "Jumlah anak harus angka.";
    if (values.has_children === false && ((count ?? 0) > 0 || (children?.length ?? 0) > 0)) all.has_children = 'Pilih "Ya" bila ada data anak.';
    if (count !== null && children && count < children.length) all.children_count = "Jumlah anak tidak boleh lebih sedikit dari data anak yang diisi.";
    setErrors(all);
    if (Object.keys(all).length === 0) save.mutate(values);
  }

  const divOptions = (divisions.data ?? []).filter((d) => !form.branch_id || d.branch_id === form.branch_id);
  const subOptions = (subdivisions.data ?? []).filter((s) => s.division_id === form.division_id);
  const childErr = (i: number, k: string) => errors[`children.${i}.${k}`];

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
      <form id="employee-form" onSubmit={submit} noValidate className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Section title="Identitas" description={employee ? undefined : "Akun dibuat dengan kata sandi awal yang wajib diganti saat login pertama."}>
          <Field label="Nama lengkap" htmlFor="full_name" error={errors.full_name} required className="sm:col-span-2">
            <Input id="full_name" value={form.full_name} onChange={(e) => set("full_name", e.target.value)} invalid={!!errors.full_name} autoFocus />
          </Field>
          <Field label="NIPY" htmlFor="nip" error={errors.nip} hint={employee ? undefined : "Kosongkan untuk NIPY otomatis."}>
            <Input
              id="nip"
              value={form.nip ?? ""}
              onChange={(e) => set("nip", e.target.value)}
              invalid={!!errors.nip}
              placeholder={employee ? "" : "Otomatis"}
              className="font-mono"
            />
          </Field>
          <Field label="Jenis kelamin" htmlFor="gender" error={errors.gender}>
            <Select id="gender" value={form.gender ?? ""} onChange={(e) => set("gender", (e.target.value || null) as FormState["gender"])} invalid={!!errors.gender}>
              <option value="">— Pilih —</option>
              {Object.entries(GENDERS).map(([v, label]) => (
                <option key={v} value={v}>
                  {label}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Nomor WA" htmlFor="phone" error={errors.phone} hint="Bisa dipakai untuk masuk.">
            <Input id="phone" inputMode="tel" value={form.phone ?? ""} onChange={(e) => set("phone", e.target.value)} invalid={!!errors.phone} placeholder="081234567890" />
          </Field>
          <Field label="Email" htmlFor="email" error={errors.email} hint="Opsional. Bisa dipakai untuk masuk.">
            <Input id="email" type="email" value={form.email ?? ""} onChange={(e) => set("email", e.target.value)} invalid={!!errors.email} />
          </Field>
          <Field label="Status pernikahan" htmlFor="marital_status" error={errors.marital_status}>
            <Select
              id="marital_status"
              value={form.marital_status ?? ""}
              onChange={(e) => set("marital_status", (e.target.value || null) as FormState["marital_status"])}
              invalid={!!errors.marital_status}
            >
              <option value="">— Pilih —</option>
              {Object.entries(MARITAL_STATUSES).map(([v, label]) => (
                <option key={v} value={v}>
                  {label}
                </option>
              ))}
            </Select>
          </Field>
        </Section>

        <Section title="Penempatan">
          {multiBranch && (
            <Field label="Cabang" htmlFor="branch_id" className="sm:col-span-2">
              <Select
                id="branch_id"
                value={form.branch_id}
                onChange={(e) => setForm((f) => ({ ...f, branch_id: e.target.value, division_id: "", subdivision_id: "" }))}
              >
                <option value="">Semua cabang</option>
                {branches.data?.map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
              </Select>
            </Field>
          )}
          <Field label="Divisi" htmlFor="division_id" error={errors.division_id} required>
            <Select
              id="division_id"
              value={form.division_id}
              onChange={(e) => setForm((f) => ({ ...f, division_id: e.target.value, subdivision_id: "" }))}
              invalid={!!errors.division_id}
            >
              <option value="">— Pilih divisi —</option>
              {divOptions.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                  {multiBranch && !form.branch_id ? ` · ${d.branch_name}` : ""}
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
          <Field label="Jabatan" htmlFor="position" error={errors.position}>
            <Input id="position" value={form.position ?? ""} onChange={(e) => set("position", e.target.value)} placeholder="Staf Lapangan" />
          </Field>
          <Field label="Shift" htmlFor="default_shift_id" error={errors.default_shift_id}>
            <Select
              id="default_shift_id"
              value={form.default_shift_id ?? ""}
              onChange={(e) => set("default_shift_id", e.target.value || null)}
              invalid={!!errors.default_shift_id}
            >
              <option value="">— Belum ditentukan —</option>
              {shifts.data?.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name} ({s.start_time} sampai {s.end_time})
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Tanggal bergabung" htmlFor="join_date" error={errors.join_date}>
            <Input id="join_date" type="date" value={form.join_date ?? ""} onChange={(e) => set("join_date", e.target.value)} invalid={!!errors.join_date} />
          </Field>
          {managesSalary && (
            <Field label="Ubar pokok (Rp)" htmlFor="base_salary" error={errors.base_salary} required>
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
          )}
          <Field label="Status" htmlFor="status">
            <Select id="status" value={form.status} onChange={(e) => set("status", e.target.value as FormState["status"])}>
              <option value="active">Aktif</option>
              <option value="inactive">Nonaktif</option>
            </Select>
          </Field>
        </Section>

        <Section title="Anak" description="Hanya terlihat oleh SDM, super admin, dan Pejuang yang bersangkutan.">
          <Field label="Memiliki anak" htmlFor="has_children" error={errors.has_children}>
            <Select
              id="has_children"
              value={form.has_children == null ? "" : form.has_children ? "ya" : "tidak"}
              onChange={(e) => set("has_children", e.target.value === "" ? null : e.target.value === "ya")}
              invalid={!!errors.has_children}
            >
              <option value="">— Pilih —</option>
              <option value="ya">Ya</option>
              <option value="tidak">Tidak</option>
            </Select>
          </Field>
          <Field label="Jumlah anak" htmlFor="children_count" error={errors.children_count}>
            <Input
              id="children_count"
              inputMode="numeric"
              value={form.children_count}
              onChange={(e) => set("children_count", e.target.value.replace(/\D/g, "").slice(0, 2))}
              invalid={!!errors.children_count}
              disabled={form.has_children === false}
            />
          </Field>
          {children === null ? (
            <div className="sm:col-span-2">{detail.isError ? <p className="text-sm text-red-700">Data anak gagal dimuat; data anak tidak akan diubah.</p> : <Loading />}</div>
          ) : (
            <div className="space-y-3 sm:col-span-2">
              {errors.children && <p className="text-xs text-red-700">{errors.children}</p>}
              {children.map((c, i) => (
                <div key={i} className="rounded-lg border border-slate-200 p-3">
                  <div className="mb-2 flex items-center justify-between">
                    <p className="text-sm font-medium text-slate-700">Anak {i + 1}</p>
                    <IconButton
                      label={`Hapus data anak ${i + 1}`}
                      onClick={() => setChildren((list) => (list ?? []).filter((_, j) => j !== i))}
                      className="hover:text-red-600"
                    >
                      <Trash2 className="size-4" />
                    </IconButton>
                  </div>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-6">
                    <Field label="Nama" htmlFor={`child-${i}-name`} error={childErr(i, "full_name")} required className="sm:col-span-4">
                      <Input id={`child-${i}-name`} value={c.full_name} onChange={(e) => setChild(i, { full_name: e.target.value })} invalid={!!childErr(i, "full_name")} />
                    </Field>
                    <Field label="Anak ke-" htmlFor={`child-${i}-order`} error={childErr(i, "child_order")} required className="sm:col-span-2">
                      <Input
                        id={`child-${i}-order`}
                        inputMode="numeric"
                        value={c.child_order ? String(c.child_order) : ""}
                        onChange={(e) => setChild(i, { child_order: Number(e.target.value.replace(/\D/g, "").slice(0, 2)) || 0 })}
                        invalid={!!childErr(i, "child_order")}
                      />
                    </Field>
                    <Field label="Jenis kelamin" htmlFor={`child-${i}-gender`} error={childErr(i, "gender")} className="sm:col-span-2">
                      <Select id={`child-${i}-gender`} value={c.gender ?? ""} onChange={(e) => setChild(i, { gender: (e.target.value || null) as ChildInput["gender"] })}>
                        <option value="">— Pilih —</option>
                        {Object.entries(GENDERS).map(([v, label]) => (
                          <option key={v} value={v}>
                            {label}
                          </option>
                        ))}
                      </Select>
                    </Field>
                    <Field label="Tempat lahir" htmlFor={`child-${i}-place`} error={childErr(i, "birth_place")} className="sm:col-span-2">
                      <Input id={`child-${i}-place`} value={c.birth_place ?? ""} onChange={(e) => setChild(i, { birth_place: e.target.value })} />
                    </Field>
                    <Field label="Tanggal lahir" htmlFor={`child-${i}-date`} error={childErr(i, "birth_date")} className="sm:col-span-2">
                      <Input
                        id={`child-${i}-date`}
                        type="date"
                        max={todayWib()}
                        value={c.birth_date ?? ""}
                        onChange={(e) => setChild(i, { birth_date: e.target.value })}
                        invalid={!!childErr(i, "birth_date")}
                      />
                    </Field>
                    <Field label="Status" htmlFor={`child-${i}-relation`} error={childErr(i, "relation")} className="sm:col-span-2">
                      <Select id={`child-${i}-relation`} value={c.relation ?? ""} onChange={(e) => setChild(i, { relation: (e.target.value || null) as ChildInput["relation"] })}>
                        <option value="">— Pilih —</option>
                        {Object.entries(CHILD_RELATIONS).map(([v, label]) => (
                          <option key={v} value={v}>
                            {label}
                          </option>
                        ))}
                      </Select>
                    </Field>
                    <Field label="Pendidikan" htmlFor={`child-${i}-education`} error={childErr(i, "education")} className="sm:col-span-2">
                      <Select
                        id={`child-${i}-education`}
                        value={c.education ?? ""}
                        onChange={(e) => setChild(i, { education: (e.target.value || null) as ChildInput["education"] })}
                      >
                        <option value="">— Pilih —</option>
                        {Object.entries(EDUCATION_LEVELS).map(([v, label]) => (
                          <option key={v} value={v}>
                            {label}
                          </option>
                        ))}
                      </Select>
                    </Field>
                    <Field label="Nama sekolah" htmlFor={`child-${i}-school`} error={childErr(i, "school_name")} className="sm:col-span-2">
                      <Input id={`child-${i}-school`} value={c.school_name ?? ""} onChange={(e) => setChild(i, { school_name: e.target.value })} />
                    </Field>
                  </div>
                </div>
              ))}
              {children.length < MAX_CHILDREN && form.has_children !== false && (
                <Button type="button" variant="secondary" size="sm" onClick={addChild}>
                  <Plus className="size-4" /> Tambah data anak
                </Button>
              )}
            </div>
          )}
        </Section>
      </form>
    </Modal>
  );
}
