import { CheckCircle2, Circle, Clock3, MinusCircle, Send, XCircle, type LucideIcon } from "lucide-react";
import { dateTime } from "@/lib/format";
import type { ApprovalStep, ApprovalStepStatus } from "@/lib/types";
import { APPROVER_ROLE, approverName } from "./request-status";

const STEP: Record<ApprovalStepStatus, { icon: LucideIcon; label: string; cls: string }> = {
  approved: { icon: CheckCircle2, label: "Disetujui", cls: "text-emerald-600 bg-emerald-50" },
  rejected: { icon: XCircle, label: "Ditolak", cls: "text-red-600 bg-red-50" },
  pending: { icon: Clock3, label: "Menunggu keputusan", cls: "text-amber-700 bg-amber-50 ring-4 ring-amber-100" },
  waiting: { icon: Circle, label: "Menunggu langkah sebelumnya", cls: "text-slate-300 bg-white" },
  skipped: { icon: MinusCircle, label: "Dilewati", cls: "text-slate-400 bg-slate-50" },
};

/**
 * Timeline persetujuan (MASTER.md §10.2): siapa yang mengajukan, lalu tiap langkah dengan
 * approver, status, waktu, dan catatan. Langkah dilewati tetap tampil agar jejaknya jelas.
 */
export function ApprovalTimeline({ steps, requester, submittedAt }: { steps: ApprovalStep[]; requester: string; submittedAt: string }) {
  return (
    <ol className="relative" aria-label="Riwayat persetujuan">
      <Node icon={Send} cls="text-primary-700 bg-primary-50" last={steps.length === 0}>
        <p className="text-sm font-medium text-slate-900">Diajukan oleh {requester}</p>
        <p className="text-xs text-slate-500">{dateTime(submittedAt)}</p>
      </Node>
      {steps.map((s, i) => {
        const meta = STEP[s.status];
        return (
          <Node key={s.step_order} icon={meta.icon} cls={meta.cls} last={i === steps.length - 1}>
            <div className="flex flex-wrap items-baseline justify-between gap-x-3">
              <p className="text-sm font-medium text-slate-900">
                {APPROVER_ROLE[s.approver_role]}
                <span className="font-normal text-slate-500"> · {approverName(s)}</span>
              </p>
              {s.acted_at && <p className="text-xs text-slate-500">{dateTime(s.acted_at)}</p>}
            </div>
            <p className={`text-xs ${s.status === "rejected" ? "text-red-700" : s.status === "approved" ? "text-emerald-700" : "text-slate-500"}`}>{meta.label}</p>
            {s.note && (
              <p className="mt-1.5 rounded-md bg-slate-50 px-2.5 py-1.5 text-sm break-words text-slate-700">
                {s.status === "skipped" ? s.note : <>&ldquo;{s.note}&rdquo;</>}
              </p>
            )}
          </Node>
        );
      })}
    </ol>
  );
}

function Node({ icon: Icon, cls, last, children }: { icon: LucideIcon; cls: string; last: boolean; children: React.ReactNode }) {
  return (
    <li className="relative flex gap-3 pb-4 last:pb-0">
      {!last && <span className="absolute top-8 bottom-0 left-[15px] w-px bg-slate-200" aria-hidden />}
      <span className={`relative z-10 flex size-8 shrink-0 items-center justify-center rounded-full ${cls}`} aria-hidden>
        <Icon className="size-4" />
      </span>
      <div className="min-w-0 flex-1 pt-1">{children}</div>
    </li>
  );
}
