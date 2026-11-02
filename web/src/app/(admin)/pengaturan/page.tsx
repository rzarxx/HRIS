"use client";

import { TriangleAlert } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { MfaCard } from "@/components/settings/mfa-card";
import { PasswordCard } from "@/components/settings/password-card";
import { SessionsCard } from "@/components/settings/sessions-card";
import { Card, Loading, PageHeader } from "@/components/ui/layout";
import { dateTime } from "@/lib/format";
import { useMe } from "@/lib/hooks";

const REQUIRED_MESSAGE: Record<string, string> = {
  PASSWORD_CHANGE_REQUIRED: "Ganti kata sandi Anda terlebih dahulu sebelum memakai fitur lain.",
  MFA_SETUP_REQUIRED: "Aktifkan verifikasi dua langkah terlebih dahulu. Fitur lain terkunci sampai langkah ini selesai.",
};

function SettingsView() {
  const { data: me } = useMe();
  const required = REQUIRED_MESSAGE[useSearchParams().get("wajib") ?? ""];

  return (
    <>
      <PageHeader title="Pengaturan" description="Akun dan keamanan." />
      {required && (
        <div role="alert" className="mb-4 flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
          <TriangleAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
          {required}
        </div>
      )}
      <div className="grid gap-4 lg:grid-cols-2">
        <div className="flex flex-col gap-4">
          <Card title="Akun saya">
            <dl className="grid grid-cols-3 gap-y-2 text-sm">
              <dt className="text-slate-500">Nama</dt>
              <dd className="col-span-2 text-slate-800">{me?.full_name ?? "-"}</dd>
              <dt className="text-slate-500">NIP</dt>
              <dd className="col-span-2 font-mono text-xs text-slate-800">{me?.nip ?? "-"}</dd>
              <dt className="text-slate-500">Email</dt>
              <dd className="col-span-2 text-slate-800">{me?.email ?? "-"}</dd>
              <dt className="text-slate-500">Jabatan</dt>
              <dd className="col-span-2 text-slate-800">{me?.position ?? "-"}</dd>
              <dt className="text-slate-500">Unit</dt>
              <dd className="col-span-2 text-slate-800">{me ? `${me.subdivision.name} · ${me.division.name}` : "-"}</dd>
              <dt className="text-slate-500">Role</dt>
              <dd className="col-span-2 text-slate-800 uppercase">{me?.role ?? "-"}</dd>
            </dl>
            <p className="mt-4 text-xs text-slate-500">Waktu ditampilkan dalam WIB, contoh: {dateTime(new Date().toISOString())}.</p>
          </Card>
          <MfaCard />
        </div>
        <div className="flex flex-col gap-4">
          <PasswordCard />
          <SessionsCard />
        </div>
      </div>
    </>
  );
}

export default function SettingsPage() {
  return (
    <Suspense fallback={<Loading />}>
      <SettingsView />
    </Suspense>
  );
}
