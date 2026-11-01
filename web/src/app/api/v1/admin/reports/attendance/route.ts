import { openDb, requireRole, route } from "@/mock-api/http";
import { readPeriod } from "@/mock-api/period";
import { attendanceReport } from "@/mock-api/queries";

export const GET = route(async (req) => {
  await requireRole(req, "sdm");
  const { year, month, divisionId } = readPeriod(req);
  return { data: attendanceReport(openDb(), year, month, divisionId), meta: { year, month } };
});
