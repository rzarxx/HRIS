import { announcementSchema } from "@/lib/schemas";
import { notFound, openDb, parseBody, requireRole, route } from "@/mock-api/http";
import { saveDb } from "@/mock-api/store";

type Ctx = { params: Promise<{ id: string }> };

export const PUT = route<Ctx>(async (req, { params }) => {
  await requireRole(req, "sdm");
  const { id } = await params;
  const input = await parseBody(req, announcementSchema);
  const db = openDb();
  const announcement = db.announcements.find((a) => a.id === id);
  if (!announcement) throw notFound("Pengumuman");
  Object.assign(announcement, input);
  saveDb();
  return { data: announcement };
});

export const DELETE = route<Ctx>(async (req, { params }) => {
  await requireRole(req, "sdm");
  const { id } = await params;
  const db = openDb();
  const idx = db.announcements.findIndex((a) => a.id === id);
  if (idx < 0) throw notFound("Pengumuman");
  db.announcements.splice(idx, 1);
  saveDb();
});
