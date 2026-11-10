import { Badge } from "@/components/ui/layout";
import type { ApprovalStep, ApproverRole, LeaveQuotaKind, LeaveType, RequestStatus } from "@/lib/types";

type Tone = "gray" | "accent" | "green" | "amber" | "red";

export const REQUEST_STATUS: Record<RequestStatus, { label: string; tone: Tone }> = {
  pending: { label: "Menunggu", tone: "amber" },
  approved: { label: "Disetujui", tone: "green" },
  rejected: { label: "Ditolak", tone: "red" },
  cancelled: { label: "Dibatalkan", tone: "gray" },
};

export const APPROVER_ROLE: Record<ApproverRole, string> = {
  subdiv: "Kepala Subdiv",
  kadiv: "Kepala Divisi",
  sdm: "SDM",
};

export const QUOTA_KIND: Record<LeaveQuotaKind, { label: string; description: string }> = {
  statutory: {
    label: "Sesuai UU Ketenagakerjaan",
    description: "12 hari per tahun, berlaku setelah 12 bulan bekerja sejak tanggal bergabung.",
  },
  custom: { label: "Diatur SDM", description: "Jatah hari per tahun ditentukan SDM untuk semua Pejuang." },
  none: { label: "Tanpa saldo", description: "Tidak memotong saldo, misalnya sakit atau izin keperluan keluarga." },
};

export function quotaSummary(t: Pick<LeaveType, "quota_kind" | "annual_quota_days">) {
  if (t.quota_kind === "statutory") return "12 hari/tahun (UU)";
  if (t.quota_kind === "custom") return `${t.annual_quota_days ?? 0} hari/tahun`;
  return "Tanpa saldo";
}

export function RequestStatusBadge({ status }: { status: RequestStatus }) {
  const s = REQUEST_STATUS[status];
  return <Badge tone={s.tone}>{s.label}</Badge>;
}

/** Langkah yang sedang menunggu keputusan, untuk menampilkan "di siapa" pengajuan berhenti. */
export function currentStep(steps: ApprovalStep[]) {
  return steps.find((s) => s.status === "pending") ?? null;
}

/** Siapa yang memegang langkah: nama kepala, atau tim SDM untuk pool. */
export function approverName(step: Pick<ApprovalStep, "approver_role" | "approver_name">) {
  return step.approver_name ?? (step.approver_role === "sdm" ? "Tim SDM" : APPROVER_ROLE[step.approver_role]);
}
