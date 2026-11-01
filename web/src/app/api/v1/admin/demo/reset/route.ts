// Khusus API sementara: kembalikan data dummy ke kondisi awal (persiapan demo).
import { requireRole, route } from "@/mock-api/http";
import { ensureAttendance } from "@/mock-api/attendance";
import { resetDb, saveDb } from "@/mock-api/store";

export const POST = route(async (req) => {
  await requireRole(req, "sdm");
  const db = resetDb();
  ensureAttendance(db);
  saveDb();
});
