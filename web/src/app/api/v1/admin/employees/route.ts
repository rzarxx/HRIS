import type { NextRequest } from "next/server";
import { employeeSchema } from "@/lib/schemas";
import type { Employee } from "@/lib/types";
import { assertEmployeeUnique } from "@/mock-api/employees";
import { intParam, openDb, parseBody, requireRole, route } from "@/mock-api/http";
import { listEmployees, nextNip, toEmployeeItem } from "@/mock-api/queries";
import { nowIso, saveDb, uuid } from "@/mock-api/store";

export const GET = route(async (req) => {
  await requireRole(req, "sdm");
  const sp = req.nextUrl.searchParams;
  const q = sp.get("q")?.trim().toLowerCase();
  const divisionId = sp.get("division_id");
  const subdivisionId = sp.get("subdivision_id");
  const status = sp.get("status");
  const page = intParam(req, "page", 1);
  const perPage = Math.min(intParam(req, "per_page", 10), 100);

  let rows = listEmployees(openDb());
  if (q) {
    rows = rows.filter((e) =>
      [e.full_name, e.nip, e.email ?? "", e.position ?? ""].some((v) => v.toLowerCase().includes(q)),
    );
  }
  if (divisionId) rows = rows.filter((e) => e.division_id === divisionId);
  if (subdivisionId) rows = rows.filter((e) => e.subdivision_id === subdivisionId);
  if (status) rows = rows.filter((e) => e.status === status);
  rows.sort((a, b) => a.full_name.localeCompare(b.full_name));

  return {
    data: rows.slice((page - 1) * perPage, page * perPage),
    meta: { page, per_page: perPage, total: rows.length },
  };
});

export const POST = route(async (req: NextRequest) => {
  await requireRole(req, "sdm");
  const input = await parseBody(req, employeeSchema);
  const db = openDb();
  const nip = (input.nip ?? nextNip(db)).toUpperCase();
  assertEmployeeUnique(db, { ...input, nip });

  const now = nowIso();
  const employee: Employee = {
    id: uuid(),
    ...input,
    nip,
    join_date: input.join_date ?? now.slice(0, 10),
    photo_url: null,
    created_at: now,
    updated_at: now,
    deleted_at: null,
  };
  db.employees.push(employee);
  saveDb();
  return { data: toEmployeeItem(db, employee) };
});
