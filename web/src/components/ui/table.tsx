"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { number } from "@/lib/format";

const alignCls = { left: "text-left", right: "text-right", center: "text-center" } as const;

// Kolom aksi ditempel ke kanan. Kolom yang lebar hanya ditempel mulai ukuran sm, agar di ponsel tidak menutupi isi tabel.
const stickyActions = {
  always: "[&_td:last-child]:bg-white [&_tr>*:last-child]:sticky [&_tr>*:last-child]:right-0",
  sm: "sm:[&_td:last-child]:bg-white sm:[&_tr>*:last-child]:sticky sm:[&_tr>*:last-child]:right-0",
} as const;
const stickyShadow = "[&_tr>*:last-child]:shadow-[-8px_0_8px_-8px_rgb(15_23_42_/_0.25)]";

export function Table({ children, actions }: { children: ReactNode; actions?: keyof typeof stickyActions }) {
  const box = useRef<HTMLDivElement>(null);
  const [scroll, setScroll] = useState({ scrollable: false, more: false });

  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const update = () => {
      const hidden = el.scrollWidth - el.clientWidth;
      const next = { scrollable: hidden > 1, more: hidden - el.scrollLeft > 1 };
      setScroll((s) => (s.scrollable === next.scrollable && s.more === next.more ? s : next));
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    if (el.firstElementChild) observer.observe(el.firstElementChild);
    el.addEventListener("scroll", update, { passive: true });
    return () => {
      observer.disconnect();
      el.removeEventListener("scroll", update);
    };
  }, []);

  return (
    <div className="relative">
      <div
        ref={box}
        tabIndex={scroll.scrollable ? 0 : undefined}
        role={scroll.scrollable ? "region" : undefined}
        aria-label={scroll.scrollable ? "Tabel, geser ke samping untuk melihat kolom lain" : undefined}
        className="relative overflow-x-auto focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary-600"
      >
        <table className={`w-full border-separate border-spacing-0 text-left text-sm ${actions ? stickyActions[actions] : ""} ${actions && scroll.more ? stickyShadow : ""}`}>
          {children}
        </table>
      </div>
      {scroll.more && actions !== "always" && (
        <div className={`pointer-events-none absolute inset-y-0 right-0 w-8 bg-gradient-to-l from-slate-900/10 to-transparent ${actions === "sm" ? "sm:hidden" : ""}`} aria-hidden />
      )}
    </div>
  );
}

export function Th({ children, align = "left", className = "" }: { children?: ReactNode; align?: "left" | "right" | "center"; className?: string }) {
  return (
    <th
      scope="col"
      className={`border-b border-slate-200 bg-slate-50 px-4 py-2.5 text-xs font-semibold tracking-wide text-slate-500 uppercase whitespace-nowrap ${alignCls[align]} ${className}`}
    >
      {children}
    </th>
  );
}

export function Td({ children, align = "left", className = "" }: { children?: ReactNode; align?: "left" | "right" | "center"; className?: string }) {
  return <td className={`border-b border-slate-100 px-4 py-2.5 text-slate-700 ${alignCls[align]} ${className}`}>{children}</td>;
}

export function Pagination({
  page,
  perPage,
  total,
  onPage,
}: {
  page: number;
  perPage: number;
  total: number;
  onPage: (p: number) => void;
}) {
  const pages = Math.max(1, Math.ceil(total / perPage));
  const from = total === 0 ? 0 : (page - 1) * perPage + 1;
  const to = Math.min(total, page * perPage);
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 px-4 py-3 text-sm text-slate-500">
      <span>
        Menampilkan {number(from)}–{number(to)} dari {number(total)}
      </span>
      <div className="flex items-center gap-1">
        <button
          type="button"
          aria-label="Halaman sebelumnya"
          disabled={page <= 1}
          onClick={() => onPage(page - 1)}
          className="inline-flex size-8 items-center justify-center rounded-md border border-slate-200 bg-white transition-colors hover:bg-slate-50 active:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-white"
        >
          <ChevronLeft className="size-4" />
        </button>
        <span className="px-2 text-slate-700">
          {page} / {pages}
        </span>
        <button
          type="button"
          aria-label="Halaman berikutnya"
          disabled={page >= pages}
          onClick={() => onPage(page + 1)}
          className="inline-flex size-8 items-center justify-center rounded-md border border-slate-200 bg-white transition-colors hover:bg-slate-50 active:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-white"
        >
          <ChevronRight className="size-4" />
        </button>
      </div>
    </div>
  );
}
