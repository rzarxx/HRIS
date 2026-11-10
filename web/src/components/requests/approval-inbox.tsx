"use client";

import { Check, CheckCircle2, X } from "lucide-react";
import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Textarea } from "@/components/ui/form";
import { Badge, Card, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { Modal } from "@/components/ui/modal";
import { api } from "@/lib/api-client";
import { dateRange, dateTime, number, timeAgo } from "@/lib/format";
import { useApprovalDetail, usePendingApprovals, useSave } from "@/lib/hooks";
import type { ApprovalDecision, ApprovalItem, RequestKind } from "@/lib/types";
import { ApprovalTimeline } from "./approval-timeline";
import { Avatar } from "./leave-list";
import { APPROVER_ROLE } from "./request-status";

const KIND: Record<RequestKind, string> = { leave: "Cuti/Izin", cash_advance: "Pinjaman", transfer: "Mutasi" };

type Decision = { item: ApprovalItem; action: "approve" | "reject" };

export function ApprovalInbox() {
  const list = usePendingApprovals();
  const [deciding, setDeciding] = useState<Decision | null>(null);
  // Antrean: yang paling lama menunggu tampil lebih dulu.
  const items = useMemo(() => [...(list.data ?? [])].sort((a, b) => a.created_at.localeCompare(b.created_at)), [list.data]);

  return (
    <>
      <PageHeader
        title="Persetujuan"
        description="Pengajuan yang menunggu keputusan Anda. Yang paling lama menunggu tampil lebih dulu."
        actions={items.length > 0 && <Badge tone="amber">{number(items.length)} menunggu</Badge>}
      />
      {list.isPending ? (
        <Card>
          <Loading />
        </Card>
      ) : list.isError ? (
        <Card>
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        </Card>
      ) : items.length === 0 ? (
        <Card>
          <div className="flex flex-col items-center gap-2 py-12 text-center">
            <span className="flex size-12 items-center justify-center rounded-full bg-primary-50">
              <CheckCircle2 className="size-6 text-primary-600" aria-hidden />
            </span>
            <p className="text-base font-semibold text-slate-900">Semua sudah diputuskan</p>
            <p className="max-w-sm text-sm text-slate-500">Tidak ada pengajuan yang menunggu Anda. Pengajuan baru dari Pejuang akan muncul di sini otomatis.</p>
          </div>
        </Card>
      ) : (
        <ul className="grid grid-cols-1 gap-3 xl:grid-cols-2">
          {items.map((a) => (
            <li key={a.id}>
              <ApprovalCard item={a} onDecide={(action) => setDeciding({ item: a, action })} />
            </li>
          ))}
        </ul>
      )}
      {deciding && <DecisionModal key={deciding.item.id} decision={deciding} onSwitch={setDeciding} onClose={() => setDeciding(null)} />}
    </>
  );
}

function ApprovalCard({ item, onDecide }: { item: ApprovalItem; onDecide: (action: Decision["action"]) => void }) {
  const s = item.summary;
  return (
    <article className="flex h-full flex-col rounded-[var(--radius-card)] border border-slate-200 bg-white transition-shadow hover:shadow-md">
      <header className="flex items-start gap-3 px-4 pt-4">
        <Avatar name={item.requester.full_name} />
        <div className="min-w-0 flex-1">
          <p className="truncate font-medium text-slate-900">{item.requester.full_name}</p>
          <p className="truncate text-xs text-slate-500">
            {item.requester.subdivision_name} · diajukan {timeAgo(item.created_at)}
          </p>
        </div>
      </header>
      <div className="flex-1 px-4 py-3">
        <p className="mb-1 text-xs font-medium text-primary-700">
          {KIND[item.request_type]} · langkah {APPROVER_ROLE[item.approver_role]}
        </p>
        {s.start_date && s.end_date && (
          <div className="flex flex-wrap items-baseline gap-x-2">
            <p className="text-base font-semibold text-slate-900">{dateRange(s.start_date, s.end_date)}</p>
            <p className="text-sm text-slate-600">
              {s.leave_type} · {number(s.days ?? 0)} hari kerja
            </p>
          </div>
        )}
        {s.reason && <p className="mt-1.5 line-clamp-3 text-sm break-words text-slate-600">{s.reason}</p>}
      </div>
      <footer className="flex items-center justify-end gap-2 border-t border-slate-100 px-4 py-3">
        <Button variant="secondary" onClick={() => onDecide("reject")} className="text-red-700" aria-label={`Tolak pengajuan ${item.requester.full_name}`}>
          <X className="size-4" aria-hidden /> Tolak
        </Button>
        <Button onClick={() => onDecide("approve")} aria-label={`Setujui pengajuan ${item.requester.full_name}`}>
          <Check className="size-4" aria-hidden /> Setujui
        </Button>
      </footer>
    </article>
  );
}

function DecisionModal({ decision, onSwitch, onClose }: { decision: Decision; onSwitch: (d: Decision) => void; onClose: () => void }) {
  const { item, action } = decision;
  const detail = useApprovalDetail(item.id);
  const [note, setNote] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const reject = action === "reject";
  const name = item.requester.full_name;

  const save = useSave((body: ApprovalDecision) => api.post(`/approvals/${item.id}/${action}`, body), {
    success: reject ? `Pengajuan ${name} ditolak.` : `Pengajuan ${name} disetujui.`,
    onDone: onClose,
    onFieldErrors: setErrors,
  });

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const text = note.trim();
    if (reject && text.length < 3) {
      setErrors({ note: "Tulis alasan penolakan, minimal 3 karakter. Alasan ini dibaca pemohon." });
      return;
    }
    setErrors({});
    save.mutate({ note: text || null });
  }

  const s = item.summary;
  return (
    <Modal
      open
      title={reject ? `Tolak pengajuan ${name}` : `Setujui pengajuan ${name}`}
      onClose={onClose}
      footer={
        <>
          <button
            type="button"
            onClick={() => onSwitch({ item, action: reject ? "approve" : "reject" })}
            className="mr-auto text-sm font-medium text-slate-500 hover:text-slate-800 hover:underline"
          >
            {reject ? "Setujui saja" : "Tolak saja"}
          </button>
          <Button variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" form="decision-form" variant={reject ? "danger" : "primary"} loading={save.isPending}>
            {reject ? "Tolak pengajuan" : "Setujui pengajuan"}
          </Button>
        </>
      }
    >
      <div className="rounded-lg bg-slate-50 px-4 py-3">
        <p className="text-xs font-medium text-slate-500">
          {KIND[item.request_type]}
          {s.leave_type ? ` · ${s.leave_type}` : ""}
        </p>
        {s.start_date && s.end_date && (
          <p className="text-lg font-semibold text-slate-900">
            {dateRange(s.start_date, s.end_date)} <span className="text-sm font-normal text-slate-600">({number(s.days ?? 0)} hari kerja)</span>
          </p>
        )}
        {s.reason && <p className="mt-1 text-sm break-words whitespace-pre-line text-slate-700">{s.reason}</p>}
        <p className="mt-1 text-xs text-slate-500">
          {item.requester.subdivision_name} · diajukan {dateTime(item.created_at)}
        </p>
      </div>

      <form id="decision-form" onSubmit={submit} noValidate className="mt-4">
        <Field
          label={reject ? "Alasan penolakan" : "Catatan"}
          htmlFor="decision_note"
          error={errors.note}
          required={reject}
          hint={reject ? "Dibaca pemohon di aplikasi." : "Opsional, terlihat di riwayat persetujuan."}
        >
          <Textarea
            id="decision_note"
            rows={3}
            maxLength={500}
            autoFocus={reject}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            invalid={!!errors.note}
            placeholder={reject ? "Contoh: jadwal bertabrakan dengan agenda yayasan, mohon ajukan tanggal lain." : ""}
          />
        </Field>
      </form>

      <h3 className="mt-5 mb-3 text-sm font-semibold text-slate-800">Riwayat persetujuan</h3>
      {detail.isPending ? (
        <Loading label="Memuat riwayat..." />
      ) : detail.isError ? (
        <ErrorState message={detail.error.message} onRetry={() => detail.refetch()} />
      ) : (
        <ApprovalTimeline steps={detail.data.steps ?? []} requester={name} submittedAt={item.created_at} />
      )}
    </Modal>
  );
}
