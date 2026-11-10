"use client";

import { Search } from "lucide-react";
import { useMemo, useState } from "react";
import { Input } from "@/components/ui/form";
import { Badge, EmptyState, ErrorState, Loading } from "@/components/ui/layout";
import { Modal } from "@/components/ui/modal";
import { dateTime, number } from "@/lib/format";
import { useAnnouncementReaders } from "@/lib/hooks";
import type { AnnouncementListItem, AnnouncementReader } from "@/lib/types";

type ReadState = "unopened" | "opened" | "read";

const STATE: Record<ReadState, { label: string; tone: "gray" | "amber" | "green" }> = {
  unopened: { label: "Belum dibuka", tone: "gray" },
  opened: { label: "Dibuka, belum dibaca", tone: "amber" },
  read: { label: "Sudah dibaca", tone: "green" },
};

const readState = (r: AnnouncementReader): ReadState => (r.read_at ? "read" : r.opened_at ? "opened" : "unopened");

/** Ringkas "12 / 40 dibaca" dengan bilah kemajuan, untuk kolom tabel. */
export function ReadProgress({ a, onOpen }: { a: AnnouncementListItem; onOpen: () => void }) {
  const pct = a.audience_count > 0 ? Math.round((a.read_count / a.audience_count) * 100) : 0;
  return (
    <button
      type="button"
      onClick={onOpen}
      className="group w-32 rounded-md p-1 text-left hover:bg-slate-100 focus-visible:outline-2 focus-visible:outline-primary-600"
      aria-label={`Lihat status baca ${a.title}: ${a.read_count} dari ${a.audience_count} sudah membaca`}
    >
      <span className="block text-xs text-slate-700">
        <strong className="font-semibold text-slate-900">{number(a.read_count)}</strong> / {number(a.audience_count)} dibaca
      </span>
      <span className="mt-1 block h-1.5 overflow-hidden rounded-full bg-slate-200" aria-hidden>
        <span className="block h-full rounded-full bg-primary-600" style={{ width: `${pct}%` }} />
      </span>
      <span className="mt-0.5 block text-[11px] text-slate-500 group-hover:text-primary-700">
        {number(a.opened_count)} sudah membuka
      </span>
    </button>
  );
}

/** Daftar Pejuang sasaran dengan status baca: belum dibuka, dibuka, sudah dibaca. */
export function ReadersModal({ a, onClose }: { a: AnnouncementListItem | null; onClose: () => void }) {
  const readers = useAnnouncementReaders(a?.id ?? null);
  const [tab, setTab] = useState<ReadState | "all">("all");
  const [q, setQ] = useState("");

  const rows = useMemo(() => readers.data ?? [], [readers.data]);
  const counts = useMemo(() => {
    const c: Record<ReadState, number> = { unopened: 0, opened: 0, read: 0 };
    for (const r of rows) c[readState(r)]++;
    return c;
  }, [rows]);
  const shown = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return rows.filter(
      (r) =>
        (tab === "all" || readState(r) === tab) &&
        (!needle || r.full_name.toLowerCase().includes(needle) || r.nip.toLowerCase().includes(needle) || r.subdivision_name.toLowerCase().includes(needle)),
    );
  }, [rows, tab, q]);

  const tabs: [ReadState | "all", string, number][] = [
    ["all", "Semua", rows.length],
    ["unopened", "Belum dibuka", counts.unopened],
    ["opened", "Belum dibaca", counts.opened],
    ["read", "Sudah dibaca", counts.read],
  ];

  return (
    <Modal open={a !== null} title={`Status baca: ${a?.title ?? ""}`} size="lg" onClose={onClose}>
      <p className="mb-3 text-sm text-slate-600">
        Sasaran: Pejuang aktif {a?.division_name ? `di divisi ${a.division_name}` : "di semua divisi"}. &ldquo;Sudah dibaca&rdquo; dicatat setelah Pejuang membuka isi di
        aplikasi dan menunggu {a?.min_read_seconds ?? 0} detik.
      </p>
      <div role="tablist" aria-label="Saring status baca" className="mb-3 flex gap-1 overflow-x-auto overflow-y-hidden border-b border-slate-200 [scrollbar-width:none]">
        {tabs.map(([key, label, n]) => (
          <button
            key={key}
            role="tab"
            type="button"
            aria-selected={tab === key}
            onClick={() => setTab(key)}
            className={`-mb-px shrink-0 border-b-2 px-3 py-2 text-sm font-medium ${
              tab === key ? "border-primary-600 text-primary-700" : "border-transparent text-slate-500 hover:text-slate-800"
            }`}
          >
            {label} <span className="ml-1 rounded-full bg-slate-100 px-1.5 text-xs text-slate-600">{number(n)}</span>
          </button>
        ))}
      </div>
      <label className="relative mb-3 block">
        <span className="sr-only">Cari nama, NIP, atau subdivisi</span>
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400" aria-hidden />
        <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Cari nama, NIP, atau subdivisi" className="pl-9" />
      </label>
      {readers.isPending ? (
        <Loading />
      ) : readers.isError ? (
        <ErrorState message={readers.error.message} onRetry={() => readers.refetch()} />
      ) : shown.length === 0 ? (
        <EmptyState title={rows.length === 0 ? "Belum ada Pejuang aktif di sasaran ini" : "Tidak ada yang cocok"} />
      ) : (
        <ul className="max-h-[50vh] divide-y divide-slate-100 overflow-y-auto rounded-md border border-slate-200">
          {shown.map((r) => {
            const s = STATE[readState(r)];
            return (
              <li key={r.employee_id} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2.5">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-slate-900">{r.full_name}</p>
                  <p className="truncate text-xs text-slate-500">
                    {r.nip} · {r.subdivision_name}
                  </p>
                </div>
                <div className="text-right">
                  <Badge tone={s.tone}>{s.label}</Badge>
                  {(r.read_at || r.opened_at) && <p className="mt-0.5 text-[11px] text-slate-500">{dateTime(r.read_at ?? r.opened_at)}</p>}
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </Modal>
  );
}
