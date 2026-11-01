// Simulasi presensi harian sampai backend presensi (Fase 2) dan aplikasi HP (Fase 3) siap.
// Setiap hari kerja yang belum punya baris akan dibuat otomatis, termasuk baris `absent`
// (alpha harus berupa baris nyata agar terhitung di rekap & laporan upah).
import { todayWib } from "@/lib/format";
import { rng } from "./seed";
import type { AttendanceRow, DB } from "./store";

export const ATTENDANCE_START = "2026-10-01";

function hash(s: string) {
  let h = 2166136261;
  for (let i = 0; i < s.length; i++) h = Math.imul(h ^ s.charCodeAt(i), 16777619);
  return h >>> 0;
}

function addDays(date: string, n: number) {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

const isWeekday = (date: string) => {
  const day = new Date(`${date}T00:00:00Z`).getUTCDay();
  return day !== 0 && day !== 6;
};

const wibIso = (date: string, minutes: number) => {
  const hh = String(Math.floor(minutes / 60)).padStart(2, "0");
  const mm = String(minutes % 60).padStart(2, "0");
  return new Date(`${date}T${hh}:${mm}:00+07:00`).toISOString();
};

function minutesNowWib(now: Date) {
  const [h, m] = now.toLocaleTimeString("en-GB", { timeZone: "Asia/Jakarta", hour12: false }).split(":").map(Number);
  return h * 60 + m;
}

/** Mengisi presensi yang belum ada hingga hari ini. Mengembalikan true bila ada perubahan. */
export function ensureAttendance(db: DB, now = new Date()): boolean {
  const today = todayWib(now);
  const nowMin = minutesNowWib(now);
  const shift = db.shifts[0];
  const [sh, sm] = shift.start_time.split(":").map(Number);
  const shiftStart = sh * 60 + sm;
  const [eh, em] = shift.end_time.split(":").map(Number);
  const shiftEnd = eh * 60 + em;

  const existing = new Set(db.attendances.map((a) => `${a.employee_id}|${a.work_date}`));
  let changed = false;

  for (const emp of db.employees) {
    if (emp.deleted_at || emp.status !== "active") continue;
    const joined = emp.join_date && emp.join_date > ATTENDANCE_START ? emp.join_date : ATTENDANCE_START;
    const profile = rng(hash(emp.id));
    const lateProb = 0.03 + profile() * 0.22;
    const absentProb = profile() * 0.05;

    for (let date = joined; date <= today; date = addDays(date, 1)) {
      if (!isWeekday(date) || existing.has(`${emp.id}|${date}`)) continue;

      const r = rng(hash(`${emp.id}|${date}`));
      const roll = r();
      let row: AttendanceRow | null = null;
      const base = { id: crypto.randomUUID(), employee_id: emp.id, work_date: date, shift_id: shift.id, risk_score: 0 };
      const isToday = date === today;

      if (roll < absentProb) {
        // Alpha baru dipastikan setelah jam pulang
        if (!isToday || nowMin >= shiftEnd) {
          row = { ...base, clock_in_at: null, clock_out_at: null, status: "absent", late_minutes: 0 };
        }
      } else if (roll < absentProb + 0.04) {
        row = { ...base, clock_in_at: null, clock_out_at: null, status: "leave", late_minutes: 0 };
      } else {
        const late = r() < lateProb;
        const inMin = late
          ? shiftStart + shift.late_tolerance_min + 1 + Math.floor(r() ** 2 * 70)
          : shiftStart - 40 + Math.floor(r() * (40 + shift.late_tolerance_min));
        const outMin = shiftEnd + Math.floor(r() * 60);
        if (!isToday || nowMin >= inMin) {
          row = {
            ...base,
            clock_in_at: wibIso(date, inMin),
            clock_out_at: isToday && nowMin < outMin ? null : wibIso(date, outMin),
            status: late ? "late" : "present",
            late_minutes: late ? inMin - shiftStart : 0,
            risk_score: Math.floor(r() * 20),
          };
        }
      }

      if (row) {
        db.attendances.push(row);
        existing.add(`${emp.id}|${date}`);
        changed = true;
      }
    }
  }
  return changed;
}
