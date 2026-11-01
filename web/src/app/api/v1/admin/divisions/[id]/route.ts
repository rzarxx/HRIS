import { divisionSchema } from "@/lib/schemas";
import { HttpError, notFound, openDb, parseBody, requireRole, route } from "@/mock-api/http";
import { saveDb } from "@/mock-api/store";

type Ctx = { params: Promise<{ id: string }> };

export const PUT = route<Ctx>(async (req, { params }) => {
  await requireRole(req, "sdm");
  const { id } = await params;
  const input = await parseBody(req, divisionSchema);
  const db = openDb();
  const division = db.divisions.find((d) => d.id === id);
  if (!division) throw notFound("Divisi");
  if (db.divisions.some((d) => d.id !== id && d.code === input.code)) {
    throw new HttpError(409, "DUPLICATE", "Kode divisi sudah dipakai.", { code: "Kode divisi sudah dipakai." });
  }
  Object.assign(division, input);
  saveDb();
  return { data: division };
});

export const DELETE = route<Ctx>(async (req, { params }) => {
  await requireRole(req, "sdm");
  const { id } = await params;
  const db = openDb();
  const idx = db.divisions.findIndex((d) => d.id === id);
  if (idx < 0) throw notFound("Divisi");
  const subs = db.subdivisions.filter((s) => s.division_id === id).length;
  if (subs > 0) {
    throw new HttpError(409, "HAS_CHILDREN", `Divisi masih memiliki ${subs} subdiv. Pindahkan atau hapus subdiv terlebih dahulu.`);
  }
  db.divisions.splice(idx, 1);
  db.announcements.forEach((a) => {
    if (a.division_id === id) a.division_id = null;
  });
  saveDb();
});
