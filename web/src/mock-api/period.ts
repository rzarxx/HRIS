import type { NextRequest } from "next/server";
import { todayWib } from "@/lib/format";
import { HttpError } from "./http";

/** Baca ?year=&month=&division_id= ; default bulan berjalan (WIB). */
export function readPeriod(req: NextRequest) {
  const sp = req.nextUrl.searchParams;
  const [ty, tm] = todayWib().split("-").map(Number);
  const year = Number(sp.get("year") ?? ty);
  const month = Number(sp.get("month") ?? tm);
  if (!Number.isInteger(year) || year < 2000 || year > 2100 || !Number.isInteger(month) || month < 1 || month > 12) {
    throw new HttpError(422, "VALIDATION", "Periode tidak valid.");
  }
  return { year, month, divisionId: sp.get("division_id") };
}
