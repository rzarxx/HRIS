// Penyimpanan sementara berbasis file JSON (web/.data/db.json).
// Hanya dipakai sampai backend Go + PostgreSQL siap; bentuk tabel mengikuti QUERY.sql.
import "server-only";
import fs from "node:fs";
import path from "node:path";
import type { Announcement, Division, Employee, Role, Subdivision, AttendanceStatus } from "@/lib/types";
import { buildSeed } from "./seed";

export interface UserRow {
  id: string;
  employee_id: string;
  email: string;
  password_hash: string;
  role: Role;
  is_active: boolean;
  last_login_at: string | null;
  created_at: string;
}

export interface ShiftRow {
  id: string;
  name: string;
  start_time: string; // HH:MM WIB
  end_time: string;
  late_tolerance_min: number;
}

export interface AttendanceRow {
  id: string;
  employee_id: string;
  work_date: string; // YYYY-MM-DD (WIB)
  shift_id: string;
  clock_in_at: string | null;
  clock_out_at: string | null;
  status: AttendanceStatus;
  late_minutes: number;
  risk_score: number;
}

export interface DB {
  version: number;
  divisions: Division[];
  subdivisions: Subdivision[];
  employees: Employee[];
  users: UserRow[];
  shifts: ShiftRow[];
  attendances: AttendanceRow[];
  announcements: Announcement[];
}

const SCHEMA_VERSION = 1;
const FILE = path.join(process.cwd(), ".data", "db.json");

const g = globalThis as unknown as { __hrisDb?: DB };

export function getDb(): DB {
  if (g.__hrisDb) return g.__hrisDb;
  try {
    const parsed = JSON.parse(fs.readFileSync(FILE, "utf8")) as DB;
    if (parsed.version === SCHEMA_VERSION) {
      g.__hrisDb = parsed;
      return parsed;
    }
  } catch {
    // file belum ada / rusak -> seed ulang
  }
  return resetDb();
}

export function saveDb() {
  if (!g.__hrisDb) return;
  fs.mkdirSync(path.dirname(FILE), { recursive: true });
  fs.writeFileSync(FILE, JSON.stringify(g.__hrisDb));
}

export function resetDb(): DB {
  g.__hrisDb = { version: SCHEMA_VERSION, ...buildSeed() };
  saveDb();
  return g.__hrisDb;
}

export const uuid = () => crypto.randomUUID();
export const nowIso = () => new Date().toISOString();
