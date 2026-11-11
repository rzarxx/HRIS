"use client";

import { defaultFooter, useBranding } from "@/lib/hooks";

/**
 * Footer seluruh halaman: teks dari super admin (Pengaturan → Footer), dengan {tahun} diganti tahun
 * berjalan oleh server. Selama belum termuat atau bila gagal, memakai teks bawaan.
 */
export function SiteFooter({ className = "" }: { className?: string }) {
  const branding = useBranding();
  return (
    <footer className={`px-4 py-6 text-center text-xs text-slate-500 ${className}`}>
      <p>{branding.data?.footer ?? defaultFooter()}</p>
    </footer>
  );
}
