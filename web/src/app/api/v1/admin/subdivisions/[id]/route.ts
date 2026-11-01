import { subdivisionSchema } from "@/lib/schemas";
import { HttpError, notFound, openDb, parseBody, requireRole, route } from "@/mock-api/http";
import { saveDb } from "@/mock-api/store";

type Ctx = { params: Promise<{ id: string }> };

export const PUT = route<Ctx>(async (req, { params }) => {
  await requireRole(req, "sdm");
  const { id } = await params;
  const input = await parseBody(req, subdivisionSchema);
  const db = openDb();
  const subdivision = db.subdivisions.find((s) => s.id === id);
  if (!subdivision) throw notFound("Subdiv");
  if (!db.divisions.some((d) => d.id === input.division_id)) {
    throw new HttpError(422, "VALIDATION", "Divisi tidak ditemukan.", { division_id: "Divisi tidak ditemukan." });
  }
  if (db.subdivisions.some((s) => s.id !== id && s.code === input.code)) {
    throw new HttpError(409, "DUPLICATE", "Kode subdiv sudah dipakai.", { code: "Kode subdiv sudah dipakai." });
  }
  Object.assign(subdivision, input);
  saveDb();
  return { data: subdivision };
});

export const DELETE = route<Ctx>(async (req, { params }) => {
  await requireRole(req, "sdm");
  const { id } = await params;
  const db = openDb();
  const idx = db.subdivisions.findIndex((s) => s.id === id);
  if (idx < 0) throw notFound("Subdiv");
  const members = db.employees.filter((e) => !e.deleted_at && e.subdivision_id === id).length;
  if (members > 0) {
    throw new HttpError(409, "HAS_CHILDREN", `Subdiv masih memiliki ${members} Pejuang. Pindahkan Pejuang terlebih dahulu.`);
  }
  db.subdivisions.splice(idx, 1);
  saveDb();
});
