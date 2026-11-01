import { openDb, requireRole, route } from "@/mock-api/http";
import { orgChart } from "@/mock-api/queries";

export const GET = route(async (req) => {
  await requireRole(req, "sdm");
  return { data: orgChart(openDb()) };
});
