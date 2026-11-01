import { NextResponse } from "next/server";
import { loginSchema } from "@/lib/schemas";
import type { CurrentUser } from "@/lib/types";
import { HttpError, issueToken, openDb, parseBody, route, setSessionCookie } from "@/mock-api/http";
import { verifyPassword } from "@/mock-api/password";
import { nowIso, saveDb } from "@/mock-api/store";

export const POST = route(async (req) => {
  const { email, password } = await parseBody(req, loginSchema);
  const db = openDb();
  const user = db.users.find((u) => u.email.toLowerCase() === email.toLowerCase());
  if (!user || !user.is_active || !verifyPassword(password, user.password_hash)) {
    throw new HttpError(401, "INVALID_CREDENTIALS", "Email atau password salah.");
  }
  // Web admin hanya untuk SDM pada fase ini (Subdiv/Kadiv menyusul di Fase 4)
  if (user.role !== "sdm") throw new HttpError(403, "FORBIDDEN", "Akun ini belum memiliki akses web admin.");

  const employee = db.employees.find((e) => e.id === user.employee_id)!;
  user.last_login_at = nowIso();
  saveDb();

  const token = await issueToken({ sub: user.id, employee_id: user.employee_id, role: user.role });
  const me: CurrentUser = {
    id: user.id,
    email: user.email,
    role: user.role,
    employee_id: employee.id,
    full_name: employee.full_name,
    position: employee.position,
  };
  const res = NextResponse.json({ access_token: token, user: me });
  setSessionCookie(res, token);
  return res;
});
