"use client";

import { Bell, LogOut, Menu, Search } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { IconButton } from "@/components/ui/button";
import { api } from "@/lib/api-client";
import { initials } from "@/lib/format";
import { useMe } from "@/lib/hooks";

const ROLE_LABEL = { sdm: "SDM",
  superadmin: "Super Admin", kadiv: "Kadiv", subdiv: "Subdiv", pejuang: "Pejuang" } as const;

export function Header({ onMenu }: { onMenu: () => void }) {
  const router = useRouter();
  const { data: me } = useMe();
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent) => {
      if (!menuRef.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, [open]);

  async function logout() {
    await api.post("/auth/logout").catch(() => {});
    window.location.href = "/login/";
  }

  return (
    <header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-slate-200 bg-white px-4">
      <IconButton label="Buka menu" onClick={onMenu} className="lg:hidden">
        <Menu className="size-5" />
      </IconButton>

      <form
        role="search"
        className="relative max-w-md flex-1"
        onSubmit={(e) => {
          e.preventDefault();
          if (q.trim()) router.push(`/master/pejuang?q=${encodeURIComponent(q.trim())}`);
        }}
      >
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden />
        <input
          type="search"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Cari Pejuang (nama / NIPY)..."
          aria-label="Cari Pejuang"
          className="h-9 w-full rounded-md border border-slate-200 bg-slate-50 pr-3 pl-9 text-sm focus:border-primary-600 focus:bg-white focus:ring-2 focus:ring-primary-100 focus:outline-none"
        />
      </form>

      <div className="ml-auto flex items-center gap-2">
        <IconButton label="Notifikasi (segera)">
          <Bell className="size-5" />
        </IconButton>
        <div className="relative" ref={menuRef}>
          <button
            type="button"
            onClick={() => setOpen((o) => !o)}
            aria-expanded={open}
            aria-haspopup="menu"
            className="flex items-center gap-2.5 rounded-md py-1 pr-2 pl-1 hover:bg-slate-100"
          >
            <span className="flex size-8 items-center justify-center rounded-full bg-secondary-400 text-xs font-semibold text-primary-900">
              {me ? initials(me.full_name) : "…"}
            </span>
            <span className="hidden text-left leading-tight sm:block">
              <span className="block text-sm font-medium text-slate-800">{me?.full_name ?? "Memuat..."}</span>
              <span className="block text-xs text-slate-500">{me ? ROLE_LABEL[me.role] : ""}</span>
            </span>
          </button>
          {open && (
            <div role="menu" className="absolute right-0 mt-1 w-56 rounded-md border border-slate-200 bg-white py-1 shadow-lg">
              <div className="border-b border-slate-100 px-3 py-2">
                <p className="truncate text-sm font-medium text-slate-800">{me?.full_name}</p>
                <p className="truncate text-xs text-slate-500">{me?.email}</p>
              </div>
              <button
                type="button"
                role="menuitem"
                onClick={logout}
                className="flex w-full items-center gap-2 px-3 py-2 text-sm text-slate-700 hover:bg-slate-50"
              >
                <LogOut className="size-4" aria-hidden /> Keluar
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
