import { Badge } from "@/components/ui/layout";
import { MONTHS } from "@/lib/format";
import type { PayrollPeriod } from "@/lib/types";

/** Status periode ubar (D67). */
export const PERIOD_STATUS: Record<PayrollPeriod["status"], { label: string; tone: "gray" | "amber" | "accent" | "green" }> = {
  draft: { label: "Draft", tone: "gray" },
  pending_approval: { label: "Menunggu persetujuan", tone: "amber" },
  final: { label: "Final", tone: "accent" },
  paid: { label: "Sudah dibayar", tone: "green" },
};

export function PeriodStatusBadge({ status }: { status: PayrollPeriod["status"] }) {
  const s = PERIOD_STATUS[status];
  return <Badge tone={s.tone}>{s.label}</Badge>;
}

export const periodLabel = (p: Pick<PayrollPeriod, "year" | "month">) => `${MONTHS[p.month - 1]} ${p.year}`;
