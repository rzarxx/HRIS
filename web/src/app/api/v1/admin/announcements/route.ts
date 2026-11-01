import { announcementSchema } from "@/lib/schemas";
import type { Announcement, AnnouncementListItem } from "@/lib/types";
import { openDb, parseBody, requireRole, route } from "@/mock-api/http";
import { nowIso, saveDb, uuid } from "@/mock-api/store";

export const GET = route(async (req) => {
  await requireRole(req, "sdm");
  const db = openDb();
  const divisions = new Map(db.divisions.map((d) => [d.id, d.name]));
  const data: AnnouncementListItem[] = [...db.announcements]
    .sort((a, b) => b.starts_at.localeCompare(a.starts_at))
    .map((a) => ({ ...a, division_name: a.division_id ? (divisions.get(a.division_id) ?? null) : null }));
  return { data };
});

export const POST = route(async (req) => {
  const claims = await requireRole(req, "sdm");
  const input = await parseBody(req, announcementSchema);
  const db = openDb();
  const announcement: Announcement = {
    id: uuid(),
    ...input,
    image_url: null,
    created_by: claims.sub,
    created_at: nowIso(),
  };
  db.announcements.push(announcement);
  saveDb();
  return { data: announcement };
});
