"use client";

import { useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, Download, FileSpreadsheet, Info, Upload } from "lucide-react";
import { useRef, useState } from "react";
import { useToast } from "@/components/ui/toast";
import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/modal";
import { Table, Td, Th } from "@/components/ui/table";
import { api, ApiError } from "@/lib/api-client";
import { downloadBase64, downloadBlob, XLSX_TYPE } from "@/lib/download";
import { dateTime, number } from "@/lib/format";
import type { EmployeeImportResult, ItemResponse } from "@/lib/types";

const MAX_BYTES = 5 * 1024 * 1024;

type NewItems = EmployeeImportResult["new_items"];

const NEW_ITEM_GROUPS: { key: keyof NewItems; label: string }[] = [
  { key: "branches", label: "Cabang" },
  { key: "divisions", label: "Divisi" },
  { key: "subdivisions", label: "Subdiv" },
  { key: "locations", label: "Lokasi kantor" },
  { key: "shifts", label: "Shift" },
];

/** Unit, lokasi, dan shift yang belum terdaftar dan ikut dibuat impor (D56). */
function NewItemsSummary({ items, applied }: { items: NewItems; applied: boolean }) {
  const groups = NEW_ITEM_GROUPS.filter((g) => items[g.key].length > 0);
  if (groups.length === 0) return null;
  return (
    <div className="space-y-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900">
      <p className="font-medium">{applied ? "Data baru yang ikut dibuat:" : "Data baru yang akan dibuat. Pastikan namanya benar, karena salah ketik akan menjadi data baru:"}</p>
      <dl className="space-y-1">
        {groups.map((g) => (
          <div key={g.key} className="flex flex-wrap gap-x-2">
            <dt className="font-medium">{g.label}:</dt>
            <dd>{items[g.key].join(", ")}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

export async function downloadImportTemplate(onError: (message: string) => void) {
  try {
    const blob = await api.blob("/admin/employees/import-template");
    downloadBlob(blob, "template-impor-pejuang.xlsx");
  } catch (err) {
    onError(err instanceof ApiError ? err.message : "Template belum bisa diunduh. Coba lagi sebentar lagi.");
  }
}

/**
 * Impor massal Pejuang dari template xlsx (D53): unggah, periksa semua baris, lalu simpan.
 * Tidak ada data yang tersimpan sampai seluruh baris sesuai.
 */
export function ImportModal({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient();
  const toast = useToast();
  const input = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [busy, setBusy] = useState<"check" | "save" | null>(null);
  const [error, setError] = useState("");
  const [result, setResult] = useState<EmployeeImportResult | null>(null);

  async function run(target: File, save: boolean) {
    setBusy(save ? "save" : "check");
    setError("");
    try {
      const res = await api.upload<ItemResponse<EmployeeImportResult>>("/admin/employees/import", target, { dry_run: save ? "false" : "true" });
      setResult(res.data);
      if (res.data.applied) {
        await qc.invalidateQueries({ queryKey: ["employees"] });
        toast.success(`${number(res.data.created + res.data.updated)} data Pejuang tersimpan.`);
      }
    } catch (err) {
      if (err instanceof ApiError && err.fields?.file) setError(err.fields.file);
      else setError(err instanceof ApiError ? err.message : "Berkas belum bisa diproses. Coba lagi sebentar lagi.");
    } finally {
      setBusy(null);
    }
  }

  function pick(next: File | undefined) {
    if (!next) return;
    setResult(null);
    if (!next.name.toLowerCase().endsWith(".xlsx")) {
      setFile(null);
      setError("Pilih berkas .xlsx dari template impor.");
      return;
    }
    if (next.size > MAX_BYTES) {
      setFile(null);
      setError("Ukuran berkas paling besar 5 MB. Bagi data menjadi beberapa berkas.");
      return;
    }
    setFile(next);
    void run(next, false);
  }

  const issues = result?.issues ?? [];
  const ready = result && !result.applied && issues.length === 0;
  const newRows = result?.applied ? result.rows.filter((r) => r.action === "create") : [];

  return (
    <Modal
      open
      size="lg"
      title="Impor data Pejuang"
      onClose={onClose}
      footer={
        result?.applied ? (
          <Button onClick={onClose}>Selesai</Button>
        ) : (
          <>
            <Button variant="secondary" onClick={onClose}>
              Batal
            </Button>
            <Button disabled={!ready || !file} loading={busy === "save"} onClick={() => file && run(file, true)}>
              {ready ? `Simpan ${number(result.total_rows)} data` : "Simpan"}
            </Button>
          </>
        )
      }
    >
      {result?.applied ? (
        <div className="space-y-4">
          <div className="flex items-start gap-2 rounded-md bg-primary-50 px-3 py-2 text-sm text-primary-700">
            <CheckCircle2 className="mt-0.5 size-4 shrink-0" aria-hidden />
            <p>
              Data tersimpan: {number(result.created)} Pejuang baru dan {number(result.updated)} data diperbarui.
            </p>
          </div>
          <NewItemsSummary items={result.new_items} applied />
          {newRows.length > 0 && result.credentials_file && (
            <>
              <p className="text-sm text-slate-600">
                Pejuang baru mendapat kata sandi awal yang berlaku 72 jam dan wajib diganti saat login pertama. Daftar ini hanya tampil sekali, jadi mohon
                diunduh sekarang lalu disampaikan kepada masing-masing Pejuang.
              </p>
              <Button variant="secondary" onClick={() => downloadBase64(result.credentials_file!, "kata-sandi-awal-pejuang.xlsx", XLSX_TYPE)}>
                <Download className="size-4" /> Unduh daftar kata sandi awal
              </Button>
              <Table>
                <thead>
                  <tr>
                    <Th>NIPY</Th>
                    <Th>Nama</Th>
                    <Th>Kata sandi awal</Th>
                    <Th>Berlaku sampai</Th>
                  </tr>
                </thead>
                <tbody>
                  {newRows.slice(0, 50).map((r) => (
                    <tr key={r.row}>
                      <Td className="font-mono text-xs">{r.nip}</Td>
                      <Td>{r.full_name}</Td>
                      <Td className="font-mono">{r.initial_password}</Td>
                      <Td className="whitespace-nowrap">{r.expires_at ? dateTime(r.expires_at) : "-"}</Td>
                    </tr>
                  ))}
                </tbody>
              </Table>
              {newRows.length > 50 && <p className="text-xs text-slate-500">Menampilkan 50 dari {number(newRows.length)} Pejuang baru. Daftar lengkap ada di berkas unduhan.</p>}
            </>
          )}
        </div>
      ) : (
        <div className="space-y-4">
          <div className="flex items-start gap-2 rounded-md bg-slate-50 px-3 py-2 text-sm text-slate-600">
            <Info className="mt-0.5 size-4 shrink-0 text-slate-400" aria-hidden />
            <div className="space-y-1">
              <p>Unduh template, isi data Pejuang di sheet Pejuang, lalu unggah kembali. Petunjuk lengkap ada di sheet Petunjuk.</p>
              <p>NIPY yang sudah terdaftar akan memperbarui datanya; baris lain menjadi Pejuang baru dengan kata sandi awal.</p>
              <p>Cabang, divisi, dan subdiv baru boleh ditulis langsung. Lokasi dan shift baru ditambahkan di sheet Lokasi dan Shift.</p>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={() => downloadImportTemplate(toast.error)}>
              <FileSpreadsheet className="size-4" /> Unduh template
            </Button>
            <Button variant="secondary" loading={busy === "check"} onClick={() => input.current?.click()}>
              <Upload className="size-4" /> {file ? "Pilih berkas lain" : "Pilih berkas .xlsx"}
            </Button>
            <input
              ref={input}
              type="file"
              accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
              className="hidden"
              onChange={(e) => {
                pick(e.target.files?.[0]);
                e.target.value = "";
              }}
            />
          </div>
          {file && <p className="text-sm text-slate-600">Berkas: {file.name}</p>}
          {error && (
            <div role="alert" className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
              {error}
            </div>
          )}
          {ready && (
            <div className="rounded-md border border-primary-100 bg-primary-50 px-3 py-2 text-sm text-primary-700">
              Semua {number(result.total_rows)} baris sudah sesuai: {number(result.created)} Pejuang baru dan {number(result.updated)} data diperbarui. Tekan
              Simpan untuk menyimpannya.
            </div>
          )}
          {ready && <NewItemsSummary items={result.new_items} applied={false} />}
          {issues.length > 0 && (
            <>
              <p className="text-sm text-slate-700">
                Ada {number(issues.length)} isian yang perlu disesuaikan. Mohon perbaiki di berkas lalu unggah ulang; belum ada data yang disimpan.
              </p>
              <Table>
                <thead>
                  <tr>
                    <Th>Sheet</Th>
                    <Th>Baris</Th>
                    <Th>Kolom</Th>
                    <Th>Keterangan</Th>
                  </tr>
                </thead>
                <tbody>
                  {issues.map((issue, i) => (
                    <tr key={i}>
                      <Td className="whitespace-nowrap">{issue.sheet}</Td>
                      <Td className="tabular-nums">{issue.row}</Td>
                      <Td className="whitespace-nowrap">{issue.column || "-"}</Td>
                      <Td>{issue.message}</Td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            </>
          )}
        </div>
      )}
    </Modal>
  );
}
