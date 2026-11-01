import "server-only";
import { HttpError } from "./http";
import type { DB } from "./store";

export function assertEmployeeUnique(db: DB, input: { nip: string; email: string | null; subdivision_id: string }, selfId?: string) {
  if (!db.subdivisions.some((s) => s.id === input.subdivision_id)) {
    throw new HttpError(422, "VALIDATION", "Subdiv tidak ditemukan.", { subdivision_id: "Subdiv tidak ditemukan." });
  }
  const others = db.employees.filter((e) => e.id !== selfId);
  if (others.some((e) => e.nip === input.nip)) {
    throw new HttpError(409, "DUPLICATE", "NIP sudah dipakai.", { nip: "NIP sudah dipakai Pejuang lain." });
  }
  if (input.email && others.some((e) => !e.deleted_at && e.email?.toLowerCase() === input.email!.toLowerCase())) {
    throw new HttpError(409, "DUPLICATE", "Email sudah dipakai.", { email: "Email sudah dipakai Pejuang lain." });
  }
}
