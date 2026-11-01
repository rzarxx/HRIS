import { ChevronLeft, ChevronRight } from "lucide-react";
import type { ReactNode } from "react";
import { number } from "@/lib/format";

const alignCls = { left: "text-left", right: "text-right", center: "text-center" } as const;

export function Table({ children }: { children: ReactNode }) {
  return (
    <div className="relative overflow-x-auto">
      <table className="w-full text-left text-sm">{children}</table>
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
          className="inline-flex size-8 items-center justify-center rounded-md border border-slate-200 bg-white disabled:opacity-40"
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
          className="inline-flex size-8 items-center justify-center rounded-md border border-slate-200 bg-white disabled:opacity-40"
        >
          <ChevronRight className="size-4" />
        </button>
      </div>
    </div>
  );
}
