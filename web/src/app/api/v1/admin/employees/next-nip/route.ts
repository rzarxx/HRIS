import { openDb, requireRole, route } from "@/mock-api/http";
import { nextNip } from "@/mock-api/queries";

export const GET = route(async (req) => {
  await requireRole(req, "sdm");
  return { data: { nip: nextNip(openDb()) } };
});
