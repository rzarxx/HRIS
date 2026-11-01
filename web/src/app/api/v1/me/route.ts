import type { CurrentUser } from "@/lib/types";
import { notFound, openDb, requireRole, route } from "@/mock-api/http";

export const GET = route(async (req) => {
  const claims = await requireRole(req);
  const db = openDb();
  const user = db.users.find((u) => u.id === claims.sub);
  const employee = db.employees.find((e) => e.id === claims.employee_id);
  if (!user || !employee) throw notFound("Akun");
  const me: CurrentUser = {
    id: user.id,
    email: user.email,
    role: user.role,
    employee_id: employee.id,
    full_name: employee.full_name,
    position: employee.position,
  };
  return { data: me };
});
