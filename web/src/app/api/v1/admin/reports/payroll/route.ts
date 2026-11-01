import { openDb, requireRole, route } from "@/mock-api/http";
import { readPeriod } from "@/mock-api/period";
import { PAYROLL_RULES, payrollReport } from "@/mock-api/queries";

export const GET = route(async (req) => {
  await requireRole(req, "sdm");
  const { year, month, divisionId } = readPeriod(req);
  return { data: payrollReport(openDb(), year, month, divisionId), meta: { year, month, rules: PAYROLL_RULES } };
});
