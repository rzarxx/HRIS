import { Badge } from "@/components/ui/layout";
import type { AnnouncementListItem as Announcement } from "@/lib/types";

export type AnnouncementState = "inactive" | "scheduled" | "live" | "ended";

export function announcementState(a: Announcement, now = new Date().toISOString()): AnnouncementState {
  if (!a.is_active) return "inactive";
  if (now < a.starts_at) return "scheduled";
  if (a.ends_at && now > a.ends_at) return "ended";
  return "live";
}

const LABEL: Record<AnnouncementState, [string, "green" | "gray" | "amber"]> = {
  live: ["Tayang", "green"],
  scheduled: ["Terjadwal", "amber"],
  ended: ["Berakhir", "gray"],
  inactive: ["Nonaktif", "gray"],
};

export function AnnouncementStatusBadge({ a }: { a: Announcement }) {
  const [label, tone] = LABEL[announcementState(a)];
  return <Badge tone={tone}>{label}</Badge>;
}
