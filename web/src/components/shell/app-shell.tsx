"use client";

import { X } from "lucide-react";
import { useEffect, useState } from "react";
import { IconButton } from "@/components/ui/button";
import { Loading } from "@/components/ui/layout";
import { api } from "@/lib/api-client";
import { useMe } from "@/lib/hooks";
import { Header } from "./header";
import { Sidebar } from "./sidebar";

const IDLE_LIMIT_MS = 30 * 60 * 1000; // MASTER.md §5.8

/** Keluar otomatis setelah 30 menit tanpa aktivitas. */
function useIdleLogout() {
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout>;
    const logout = async () => {
      await api.post("/auth/logout").catch(() => {});
      window.location.href = "/login/";
    };
    const reset = () => {
      clearTimeout(timer);
      timer = setTimeout(logout, IDLE_LIMIT_MS);
    };
    const events = ["mousedown", "keydown", "scroll", "touchstart"] as const;
    events.forEach((e) => window.addEventListener(e, reset, { passive: true }));
    reset();
    return () => {
      clearTimeout(timer);
      events.forEach((e) => window.removeEventListener(e, reset));
    };
  }, []);
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const me = useMe();
  useIdleLogout();

  // Karena web statis, penjagaan ini hanya kenyamanan; penegakan akses ada di API.
  // Bila sesi tidak valid, api-client mengarahkan ke halaman login.
  if (me.isPending || me.isError) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Loading label={me.isError ? "Mengalihkan ke halaman masuk..." : "Memuat..."} />
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      <aside className="fixed inset-y-0 left-0 z-40 hidden w-64 border-r border-slate-200 bg-white lg:block">
        <Sidebar permissions={me.data.permissions} />
      </aside>

      {mobileOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <div className="absolute inset-0 bg-slate-900/40" onClick={() => setMobileOpen(false)} aria-hidden />
          <aside className="absolute inset-y-0 left-0 w-72 bg-white shadow-xl">
            <IconButton label="Tutup menu" onClick={() => setMobileOpen(false)} className="absolute top-3 right-3">
              <X className="size-5" />
            </IconButton>
            <Sidebar permissions={me.data.permissions} onNavigate={() => setMobileOpen(false)} />
          </aside>
        </div>
      )}

      <div className="lg:pl-64">
        <Header onMenu={() => setMobileOpen(true)} />
        <main className="mx-auto max-w-7xl p-4 sm:p-6">{children}</main>
      </div>
    </div>
  );
}
