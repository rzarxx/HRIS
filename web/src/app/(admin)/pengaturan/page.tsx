"use client";

import { RotateCcw } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, PageHeader } from "@/components/ui/layout";
import { ConfirmDialog } from "@/components/ui/modal";
import { api } from "@/lib/api-client";
import { dateTime } from "@/lib/format";
import { useMe, useSave } from "@/lib/hooks";

const USING_MOCK_API = !process.env.NEXT_PUBLIC_API_URL;

export default function SettingsPage() {
  const { data: me } = useMe();
  const [confirm, setConfirm] = useState(false);
  const reset = useSave(() => api.post("/admin/demo/reset"), {
    success: "Data dummy dikembalikan ke kondisi awal.",
    onDone: () => setConfirm(false),
  });

  return (
    <>
      <PageHeader title="Pengaturan" description="Akun dan konfigurasi sistem." />
      <div className="grid gap-4 lg:grid-cols-2">
        <Card title="Akun saya">
          <dl className="grid grid-cols-3 gap-y-2 text-sm">
            <dt className="text-slate-500">Nama</dt>
            <dd className="col-span-2 text-slate-800">{me?.full_name ?? "-"}</dd>
            <dt className="text-slate-500">Email</dt>
            <dd className="col-span-2 text-slate-800">{me?.email ?? "-"}</dd>
            <dt className="text-slate-500">Jabatan</dt>
            <dd className="col-span-2 text-slate-800">{me?.position ?? "-"}</dd>
            <dt className="text-slate-500">Role</dt>
            <dd className="col-span-2 text-slate-800 uppercase">{me?.role ?? "-"}</dd>
          </dl>
          <p className="mt-4 text-xs text-slate-500">Ganti password, refresh token, dan audit log hadir di Fase 1. Waktu ditampilkan dalam WIB, contoh: {dateTime(new Date().toISOString())}.</p>
        </Card>

        {USING_MOCK_API && (
          <Card title="Data pengembangan">
            <p className="text-sm text-slate-600">
              Aplikasi saat ini memakai API sementara dengan data dummy. Gunakan tombol ini untuk mengembalikan data ke kondisi awal sebelum demo atau
              pengujian. Semua perubahan (tambah/ubah/hapus) akan hilang.
            </p>
            <Button variant="secondary" className="mt-4" onClick={() => setConfirm(true)}>
              <RotateCcw className="size-4" /> Reset data dummy
            </Button>
          </Card>
        )}
      </div>

      <ConfirmDialog
        open={confirm}
        title="Reset data dummy?"
        message="Semua divisi, subdiv, Pejuang, pengumuman, dan presensi akan dikembalikan ke data awal."
        confirmLabel="Reset"
        loading={reset.isPending}
        onConfirm={() => reset.mutate(undefined)}
        onClose={() => setConfirm(false)}
      />
    </>
  );
}
