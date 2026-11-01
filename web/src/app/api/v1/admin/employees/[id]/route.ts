import { employeeSchema } from "@/lib/schemas";
import { HttpError, notFound, openDb, parseBody, requireRole, route } from "@/mock-api/http";
import { toEmployeeItem } from "@/mock-api/queries";
import { assertEmployeeUnique } from "@/mock-api/employees";
import { nowIso, saveDb } from "@/mock-api/store";

type Ctx = { params: Promise<{ id: string }> };

function find(id: string) {
  const db = openDb();
  const employee = db.employees.find((e) => e.id === id && !e.deleted_at);
  if (!employee) throw notFound("Pejuang");
  return { db, employee };
}

export const GET = route<Ctx>(async (req, { params }) => {
  await requireRole(req, "sdm");
  const { db, employee } = find((await params).id);
  return { data: toEmployeeItem(db, employee) };
});

export const PUT = route<Ctx>(async (req, { params }) => {
  await requireRole(req, "sdm");
  const { db, employee } = find((await params).id);
  const input = await parseBody(req, employeeSchema);
  const nip = (input.nip ?? employee.nip).toUpperCase();
  assertEmployeeUnique(db, { ...input, nip }, employee.id);
  Object.assign(employee, { ...input, nip, join_date: input.join_date ?? employee.join_date, updated_at: nowIso() });
  saveDb();
  return { data: toEmployeeItem(db, employee) };
});

/** Soft delete (ERD: employees memakai deleted_at). */
export const DELETE = route<Ctx>(async (req, { params }) => {
  const claims = await requireRole(req, "sdm");
  const { db, employee } = find((await params).id);
  if (employee.id === claims.employee_id) {
    throw new HttpError(409, "SELF_DELETE", "Anda tidak dapat menghapus data diri sendiri.");
  }
  employee.deleted_at = nowIso();
  db.subdivisions.forEach((s) => {
    if (s.head_employee_id === employee.id) s.head_employee_id = null;
  });
  db.divisions.forEach((d) => {
    if (d.head_employee_id === employee.id) d.head_employee_id = null;
  });
  saveDb();
});
