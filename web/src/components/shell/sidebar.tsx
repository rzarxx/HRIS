"use client";

import { ChevronDown } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { Logo } from "@/components/logo";
import { visibleNav, type NavItem } from "./nav";

const isActive = (pathname: string, href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));

function Group({ item, pathname, onNavigate }: { item: NavItem; pathname: string; onNavigate?: () => void }) {
  const hasActiveChild = item.children!.some((c) => isActive(pathname, c.href));
  const [open, setOpen] = useState(hasActiveChild);
  const expanded = open || hasActiveChild;
  const Icon = item.icon;
  return (
    <li>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={expanded}
        className={`flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium ${
          hasActiveChild ? "text-slate-900" : "text-slate-600 hover:bg-slate-100 hover:text-slate-900"
        }`}
      >
        <Icon className="size-4 shrink-0" aria-hidden />
        <span className="flex-1 text-left">{item.label}</span>
        <ChevronDown className={`size-4 text-slate-400 transition-transform ${expanded ? "rotate-180" : ""}`} aria-hidden />
      </button>
      {expanded && (
        <ul className="mt-0.5 mb-1 ml-5 border-l border-slate-200 pl-3">
          {item.children!.map((c) => {
            const active = isActive(pathname, c.href);
            return (
              <li key={c.href}>
                <Link
                  href={c.href}
                  onClick={onNavigate}
                  aria-current={active ? "page" : undefined}
                  className={`flex items-center justify-between rounded-md px-2.5 py-1.5 text-sm ${
                    active ? "bg-primary-50 font-medium text-primary-700 shadow-[inset_3px_0_0_var(--color-secondary-400)]" : "text-slate-600 hover:bg-slate-100 hover:text-slate-900"
                  }`}
                >
                  {c.label}
                  {c.soon && <span className="text-[10px] font-medium text-slate-400">Segera</span>}
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </li>
  );
}

export function Sidebar({ permissions, onNavigate }: { permissions: string[]; onNavigate?: () => void }) {
  const pathname = usePathname();
  return (
    <div className="flex h-full flex-col">
      <div className="flex h-14 items-center border-b border-slate-200 px-4">
        <Logo />
      </div>
      <nav aria-label="Menu utama" className="flex-1 overflow-y-auto p-3">
        <ul className="space-y-0.5">
          {visibleNav(permissions).map((item) => {
            if (item.children) return <Group key={item.label} item={item} pathname={pathname} onNavigate={onNavigate} />;
            const active = isActive(pathname, item.href!);
            const Icon = item.icon;
            return (
              <li key={item.href}>
                <Link
                  href={item.href!}
                  onClick={onNavigate}
                  aria-current={active ? "page" : undefined}
                  className={`flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium ${
                    active ? "bg-primary-50 text-primary-700 shadow-[inset_3px_0_0_var(--color-secondary-400)]" : "text-slate-600 hover:bg-slate-100 hover:text-slate-900"
                  }`}
                >
                  <Icon className="size-4 shrink-0" aria-hidden />
                  {item.label}
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>
      <div className="border-t border-slate-200 px-4 py-3 text-[11px] text-slate-400">Versi 0.1 · Fase 0</div>
    </div>
  );
}
