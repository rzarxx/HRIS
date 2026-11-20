"use client";

import { TriangleAlert } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { FooterCard } from "@/components/settings/footer-card";
import { MfaCard } from "@/components/settings/mfa-card";
import { PasswordCard } from "@/components/settings/password-card";
import { StorageCard } from "@/components/settings/storage-card";
import { ProfileCard } from "@/components/settings/profile-card";
import { SessionsCard } from "@/components/settings/sessions-card";
import { Loading, PageHeader } from "@/components/ui/layout";
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
      <PageHeader title="Pengaturan" description="Akun, keamanan, dan pengaturan aplikasi." />
      {required && (
        <div role="alert" className="mb-4 flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
          <TriangleAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
          {required}
        </div>
      )}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="flex flex-col gap-4">
          <ProfileCard />
          <MfaCard />
        </div>
        <div className="flex flex-col gap-4">
          <PasswordCard />
          <SessionsCard />
          {me?.permissions.includes("settings:manage") && <StorageCard />}
          {me?.permissions.includes("settings:manage") && <FooterCard />}
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
