"use client";

import { Pencil, Plus, Search, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { Button, IconButton } from "@/components/ui/button";
import { Field, Input, Select, Textarea } from "@/components/ui/form";
import { Badge, Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog, Modal } from "@/components/ui/modal";
import { Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { number } from "@/lib/format";
import { useOfficeLocations, useSave } from "@/lib/hooks";
import { locationSchema, parseCoordinate, parsePolygon, validate, type LocationForm } from "@/lib/schemas";
import type { OfficeLocation } from "@/lib/types";

const EMPTY: LocationForm = { name: "", latitude: "", longitude: "", radius_m: "100", polygon: "", is_active: true };

function toForm(l: OfficeLocation): LocationForm {
  return {
    name: l.name,
    latitude: String(l.latitude),
    longitude: String(l.longitude),
    radius_m: String(l.radius_m),
    polygon: (l.polygon ?? []).map((p) => `${p.latitude}, ${p.longitude}`).join("\n"),
    is_active: l.is_active,
  };
}

export default function OfficeLocationsPage() {
  const list = useOfficeLocations();
  const [q, setQ] = useState("");
  const [editing, setEditing] = useState<OfficeLocation | "new" | null>(null);
  const [form, setForm] = useState<LocationForm>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [deleting, setDeleting] = useState<OfficeLocation | null>(null);

  const rows = useMemo(() => {
    const term = q.trim().toLowerCase();
    return (list.data ?? []).filter((l) => !term || l.name.toLowerCase().includes(term));
  }, [list.data, q]);

  const save = useSave(
    (v: LocationForm) => {
      const body = {
        name: v.name,
        latitude: parseCoordinate(v.latitude),
        longitude: parseCoordinate(v.longitude),
        radius_m: Number(v.radius_m),
        polygon: parsePolygon(v.polygon),
        is_active: v.is_active,
      };
      return editing && editing !== "new" ? api.patch(`/admin/office-locations/${editing.id}`, body) : api.post("/admin/office-locations", body);
    },
    { success: "Lokasi kantor tersimpan.", onDone: () => setEditing(null), onFieldErrors: setErrors },
  );
  const remove = useSave((id: string) => api.del(`/admin/office-locations/${id}`), {
    success: "Lokasi kantor dihapus.",
    onDone: () => setDeleting(null),
  });

  function open(l: OfficeLocation | "new") {
    setErrors({});
    setForm(l === "new" ? EMPTY : toForm(l));
    setEditing(l);
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const fieldErrors = validate(locationSchema, form);
    setErrors(fieldErrors ?? {});
    if (!fieldErrors) save.mutate(form);
  }

  return (
    <>
      <PageHeader
        title="Lokasi Kantor"
        description="Batas area absen. Pejuang hanya bisa absen di lokasi yang ditugaskan kepadanya."
        actions={
          <Button onClick={() => open("new")}>
            <Plus className="size-4" /> Tambah lokasi
          </Button>
        }
      />
      <Card bodyClassName="">
        <div className="border-b border-slate-100 p-4">
          <div className="relative w-full max-w-xs">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden />
            <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Cari nama lokasi..." aria-label="Cari lokasi" className="pl-9" />
          </div>
        </div>
        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : rows.length === 0 ? (
          <EmptyState title="Lokasi tidak ditemukan" description="Tambahkan lokasi kantor pertama atau ubah pencarian." />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>Nama lokasi</Th>
                <Th>Koordinat</Th>
                <Th>Batas area</Th>
                <Th align="right">Pejuang</Th>
                <Th>Status</Th>
                <Th align="right">
                  <span className="sr-only">Aksi</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {rows.map((l) => (
                <tr key={l.id} className="hover:bg-slate-50">
                  <Td className="font-medium text-slate-900">{l.name}</Td>
                  <Td className="font-mono text-xs whitespace-nowrap">
                    {l.latitude}, {l.longitude}
                  </Td>
                  <Td>{l.polygon ? `Poligon ${l.polygon.length} titik` : `Radius ${number(l.radius_m)} m`}</Td>
                  <Td align="right">{number(l.employee_count)}</Td>
                  <Td>{l.is_active ? <Badge tone="green">Aktif</Badge> : <Badge>Nonaktif</Badge>}</Td>
                  <Td align="right" className="whitespace-nowrap">
                    <IconButton label={`Ubah ${l.name}`} onClick={() => open(l)}>
                      <Pencil className="size-4" />
                    </IconButton>
                    <IconButton label={`Hapus ${l.name}`} onClick={() => setDeleting(l)} className="hover:text-red-600">
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
        title={editing === "new" ? "Tambah lokasi kantor" : "Ubah lokasi kantor"}
        onClose={() => setEditing(null)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setEditing(null)}>
              Batal
            </Button>
            <Button type="submit" form="location-form" loading={save.isPending}>
              Simpan
            </Button>
          </>
        }
      >
        <form id="location-form" onSubmit={submit} noValidate className="grid gap-4 sm:grid-cols-2">
          <Field label="Nama lokasi" htmlFor="name" error={errors.name} required className="sm:col-span-2">
            <Input id="name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} invalid={!!errors.name} placeholder="Kantor Pusat" />
          </Field>
          <Field label="Lintang (latitude)" htmlFor="latitude" error={errors.latitude} required>
            <Input id="latitude" inputMode="decimal" value={form.latitude} onChange={(e) => setForm({ ...form, latitude: e.target.value })} invalid={!!errors.latitude} placeholder="-6.200100" />
          </Field>
          <Field label="Bujur (longitude)" htmlFor="longitude" error={errors.longitude} required>
            <Input id="longitude" inputMode="decimal" value={form.longitude} onChange={(e) => setForm({ ...form, longitude: e.target.value })} invalid={!!errors.longitude} placeholder="106.816600" />
          </Field>
          <Field label="Radius (meter)" htmlFor="radius" error={errors.radius_m} hint="10 sampai 5000 meter." required>
            <Input id="radius" inputMode="numeric" value={form.radius_m} onChange={(e) => setForm({ ...form, radius_m: e.target.value })} invalid={!!errors.radius_m} />
          </Field>
          <Field label="Status" htmlFor="status">
            <Select id="status" value={form.is_active ? "1" : "0"} onChange={(e) => setForm({ ...form, is_active: e.target.value === "1" })}>
              <option value="1">Aktif</option>
              <option value="0">Nonaktif</option>
            </Select>
          </Field>
          <Field
            label="Poligon area (opsional)"
            htmlFor="polygon"
            error={errors.polygon}
            hint="Satu titik per baris: lintang, bujur. Minimal 3 titik. Bila diisi, poligon menggantikan radius."
            className="sm:col-span-2"
          >
            <Textarea
              id="polygon"
              rows={4}
              value={form.polygon}
              onChange={(e) => setForm({ ...form, polygon: e.target.value })}
              invalid={!!errors.polygon}
              placeholder={"-6.2001, 106.8166\n-6.2001, 106.8176\n-6.2011, 106.8176"}
              className="font-mono text-xs"
            />
          </Field>
        </form>
      </Modal>

      <ConfirmDialog
        open={deleting !== null}
        title="Hapus lokasi kantor?"
        message={
          <>
            <strong className="text-slate-800">{deleting?.name}</strong> akan dihapus permanen. Lokasi yang masih ditugaskan ke Pejuang tidak dapat dihapus.
          </>
        }
        loading={remove.isPending}
        onConfirm={() => deleting && remove.mutate(deleting.id)}
        onClose={() => setDeleting(null)}
      />
    </>
  );
}
