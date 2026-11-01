import { divisionSchema } from "@/lib/schemas";
import { HttpError, openDb, parseBody, requireRole, route } from "@/mock-api/http";
import { listDivisions } from "@/mock-api/queries";
import { nowIso, saveDb, uuid } from "@/mock-api/store";

export const GET = route(async (req) => {
  await requireRole(req, "sdm");
  return { data: listDivisions(openDb()) };
});

export const POST = route(async (req) => {
  await requireRole(req, "sdm");
  const input = await parseBody(req, divisionSchema);
  const db = openDb();
  if (db.divisions.some((d) => d.code === input.code)) {
    throw new HttpError(409, "DUPLICATE", "Kode divisi sudah dipakai.", { code: "Kode divisi sudah dipakai." });
  }
  const division = { id: uuid(), ...input, created_at: nowIso() };
  db.divisions.push(division);
  saveDb();
  return { data: division };
});
