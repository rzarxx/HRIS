import { subdivisionSchema } from "@/lib/schemas";
import { HttpError, openDb, parseBody, requireRole, route } from "@/mock-api/http";
import { listSubdivisions } from "@/mock-api/queries";
import { nowIso, saveDb, uuid } from "@/mock-api/store";

export const GET = route(async (req) => {
  await requireRole(req, "sdm");
  const divisionId = req.nextUrl.searchParams.get("division_id");
  const rows = listSubdivisions(openDb());
  return { data: divisionId ? rows.filter((s) => s.division_id === divisionId) : rows };
});

export const POST = route(async (req) => {
  await requireRole(req, "sdm");
  const input = await parseBody(req, subdivisionSchema);
  const db = openDb();
  if (!db.divisions.some((d) => d.id === input.division_id)) {
    throw new HttpError(422, "VALIDATION", "Divisi tidak ditemukan.", { division_id: "Divisi tidak ditemukan." });
  }
  if (db.subdivisions.some((s) => s.code === input.code)) {
    throw new HttpError(409, "DUPLICATE", "Kode subdiv sudah dipakai.", { code: "Kode subdiv sudah dipakai." });
  }
  const subdivision = { id: uuid(), ...input, created_at: nowIso() };
  db.subdivisions.push(subdivision);
  saveDb();
  return { data: subdivision };
});
