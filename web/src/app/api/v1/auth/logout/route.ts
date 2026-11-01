import { NextResponse } from "next/server";
import { route, SESSION_COOKIE } from "@/mock-api/http";

export const POST = route(async () => {
  const res = new NextResponse(null, { status: 204 });
  res.cookies.delete(SESSION_COOKIE);
  return res;
});
