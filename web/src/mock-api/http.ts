import "server-only";
import { NextResponse, type NextRequest } from "next/server";
import { SignJWT, jwtVerify } from "jose";
import { ZodError, type ZodType, type ZodTypeDef } from "zod";
import type { Role } from "@/lib/types";
import { ensureAttendance } from "./attendance";
import { getDb, saveDb } from "./store";

export const SESSION_COOKIE = "hris_session";
const ACCESS_TTL_SEC = 60 * 60 * 8; // fase awal: 8 jam; refresh token menyusul di Fase 1
const secret = new TextEncoder().encode(process.env.JWT_SECRET ?? "dev-only-secret-ganti-di-produksi");

export class HttpError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public fields?: Record<string, string>,
  ) {
    super(message);
  }
}

export const notFound = (what = "Data") => new HttpError(404, "NOT_FOUND", `${what} tidak ditemukan.`);

/** Membungkus route handler: format error seragam { error: { code, message } } (DESIGN.md §5). */
export function route<C = unknown>(fn: (req: NextRequest, ctx: C) => Promise<unknown>) {
  return async (req: NextRequest, ctx: C) => {
    try {
      const result = await fn(req, ctx);
      if (result instanceof Response) return result;
      if (result === undefined) return new NextResponse(null, { status: 204 });
      return NextResponse.json(result);
    } catch (e) {
      if (e instanceof HttpError) {
        return NextResponse.json({ error: { code: e.code, message: e.message, fields: e.fields } }, { status: e.status });
      }
      console.error(e);
      return NextResponse.json({ error: { code: "INTERNAL", message: "Terjadi kesalahan pada server." } }, { status: 500 });
    }
  };
}

export async function parseBody<T>(req: NextRequest, schema: ZodType<T, ZodTypeDef, unknown>): Promise<T> {
  const raw = await req.json().catch(() => {
    throw new HttpError(400, "BAD_JSON", "Body request bukan JSON yang valid.");
  });
  try {
    return schema.parse(raw);
  } catch (e) {
    if (e instanceof ZodError) {
      const fields: Record<string, string> = {};
      for (const issue of e.issues) fields[issue.path.join(".")] ??= issue.message;
      throw new HttpError(422, "VALIDATION", "Periksa kembali isian form.", fields);
    }
    throw e;
  }
}

export interface Claims {
  sub: string;
  employee_id: string;
  role: Role;
}

export async function issueToken(claims: Claims) {
  return new SignJWT({ employee_id: claims.employee_id, role: claims.role })
    .setProtectedHeader({ alg: "HS256" })
    .setSubject(claims.sub)
    .setIssuedAt()
    .setExpirationTime(`${ACCESS_TTL_SEC}s`)
    .sign(secret);
}

export function setSessionCookie(res: NextResponse, token: string) {
  res.cookies.set(SESSION_COOKIE, token, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production" && process.env.INSECURE_COOKIE !== "1",
    path: "/",
    maxAge: ACCESS_TTL_SEC,
  });
}

/** Verifikasi JWT dari cookie / header Authorization dan cek role. */
export async function requireRole(req: NextRequest, ...roles: Role[]): Promise<Claims> {
  const header = req.headers.get("authorization");
  const token = header?.startsWith("Bearer ") ? header.slice(7) : req.cookies.get(SESSION_COOKIE)?.value;
  if (!token) throw new HttpError(401, "UNAUTHENTICATED", "Silakan login terlebih dahulu.");
  let claims: Claims;
  try {
    const { payload } = await jwtVerify(token, secret);
    claims = { sub: payload.sub!, employee_id: payload.employee_id as string, role: payload.role as Role };
  } catch {
    throw new HttpError(401, "TOKEN_INVALID", "Sesi berakhir, silakan login kembali.");
  }
  if (roles.length && !roles.includes(claims.role)) {
    throw new HttpError(403, "FORBIDDEN", "Anda tidak memiliki akses ke fitur ini.");
  }
  return claims;
}

/** Ambil DB dan pastikan presensi simulasi sudah terisi sampai hari ini. */
export function openDb() {
  const db = getDb();
  if (ensureAttendance(db)) saveDb();
  return db;
}

export function intParam(req: NextRequest, key: string, fallback: number) {
  const n = Number(req.nextUrl.searchParams.get(key));
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : fallback;
}
