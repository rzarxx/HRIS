// Padanan query di QUERY.sql untuk API sementara. Nomor query disebut di setiap fungsi
// agar mudah dipindahkan ke sqlc di backend Go.
import "server-only";
import { todayWib } from "@/lib/format";
import type {
  AttendanceReportRow,
  DivisionWithStats,
  Employee,
  EmployeeListItem,
  OrgChartData,
  PayrollReportRow,
  PayrollRules,
  SubdivisionWithStats,
} from "@/lib/types";
import type { DB } from "./store";

// Rumus sementara (QUERY.sql #9) - menunggu keputusan SDM (PRD §12).
export const PAYROLL_RULES: PayrollRules = {
  work_days_divisor: 26,
  late_penalty: 5_000,
  note: "Rumus sementara: potongan alpha = gaji pokok / 26 per hari, potongan telat = Rp5.000 per kejadian. Menunggu keputusan SDM.",
};

const isLive = (e: Employee) => !e.deleted_at;
const isActive = (e: Employee) => !e.deleted_at && e.status === "active";

function maps(db: DB) {
  const division = new Map(db.divisions.map((d) => [d.id, d]));
  const subdivision = new Map(db.subdivisions.map((s) => [s.id, s]));
  const employee = new Map(db.employees.map((e) => [e.id, e]));
  return { division, subdivision, employee };
}

export function listDivisions(db: DB): DivisionWithStats[] {
  const { employee } = maps(db);
  return db.divisions
    .map((d) => {
      const subs = db.subdivisions.filter((s) => s.division_id === d.id);
      const subIds = new Set(subs.map((s) => s.id));
      return {
        ...d,
        subdivision_count: subs.length,
        employee_count: db.employees.filter((e) => isActive(e) && subIds.has(e.subdivision_id)).length,
        head_name: d.head_employee_id ? (employee.get(d.head_employee_id)?.full_name ?? null) : null,
      };
    })
    .sort((a, b) => a.name.localeCompare(b.name));
}

export function listSubdivisions(db: DB): SubdivisionWithStats[] {
  const { division, employee } = maps(db);
  return db.subdivisions
    .map((s) => ({
      ...s,
      division_name: division.get(s.division_id)?.name ?? "-",
      employee_count: db.employees.filter((e) => isActive(e) && e.subdivision_id === s.id).length,
      head_name: s.head_employee_id ? (employee.get(s.head_employee_id)?.full_name ?? null) : null,
    }))
    .sort((a, b) => a.division_name.localeCompare(b.division_name) || a.name.localeCompare(b.name));
}

export function toEmployeeItem(db: DB, e: Employee): EmployeeListItem {
  const { division, subdivision } = maps(db);
  const sub = subdivision.get(e.subdivision_id);
  return {
    ...e,
    subdivision_name: sub?.name ?? "-",
    division_id: sub?.division_id ?? "",
    division_name: (sub && division.get(sub.division_id)?.name) ?? "-",
  };
}

export function listEmployees(db: DB) {
  return db.employees.filter(isLive).map((e) => toEmployeeItem(db, e));
}

/** QUERY.sql #1 & #2 + kartu ringkasan dashboard. */
export function orgChart(db: DB): OrgChartData {
  const today = todayWib();
  const subs = listSubdivisions(db);
  const todays = db.attendances.filter((a) => a.work_date === today);
  const activeIds = new Set(db.employees.filter(isActive).map((e) => e.id));
  const divisionsSorted = [...db.divisions].sort((a, b) => a.name.localeCompare(b.name));

  return {
    summary: {
      employees: activeIds.size,
      divisions: db.divisions.length,
      subdivisions: db.subdivisions.length,
      present_today: todays.filter((a) => activeIds.has(a.employee_id) && (a.status === "present" || a.status === "late")).length,
      late_today: todays.filter((a) => activeIds.has(a.employee_id) && a.status === "late").length,
      work_date: today,
    },
    per_subdivision: subs.map((s) => ({
      subdivision_id: s.id,
      subdivision: s.name,
      division_id: s.division_id,
      division: s.division_name,
      employees: s.employee_count,
    })),
    per_division: divisionsSorted.map((d) => ({
      division_id: d.id,
      division: d.name,
      subdivisions: subs.filter((s) => s.division_id === d.id).length,
      employees: subs.filter((s) => s.division_id === d.id).reduce((n, s) => n + s.employee_count, 0),
    })),
  };
}

function monthRows(db: DB, year: number, month: number, divisionId?: string | null) {
  const prefix = `${year}-${String(month).padStart(2, "0")}-`;
  const byEmployee = new Map<string, DB["attendances"]>();
  for (const a of db.attendances) {
    if (!a.work_date.startsWith(prefix)) continue;
    const list = byEmployee.get(a.employee_id) ?? [];
    list.push(a);
    byEmployee.set(a.employee_id, list);
  }
  const monthEnd = `${prefix}31`;
  return (
    listEmployees(db)
      .filter((e) => !divisionId || e.division_id === divisionId)
      .map((e) => ({ employee: e, records: byEmployee.get(e.id) ?? [] }))
      // Masuk periode bila punya presensi, atau aktif dan sudah bergabung sebelum akhir bulan.
      // Pejuang nonaktif / yang baru bergabung bulan berikutnya tidak boleh muncul dengan gaji penuh.
      .filter(({ employee: e, records }) => records.length > 0 || (e.status === "active" && (e.join_date ?? "") <= monthEnd))
      .sort((a, b) => a.employee.full_name.localeCompare(b.employee.full_name))
  );
}

/** QUERY.sql #8 - rekap presensi bulanan per Pejuang. */
export function attendanceReport(db: DB, year: number, month: number, divisionId?: string | null): AttendanceReportRow[] {
  return monthRows(db, year, month, divisionId).map(({ employee: e, records }) => {
    const count = (s: string) => records.filter((r) => r.status === s).length;
    const late = count("late");
    const present = count("present") + late;
    const leave = count("leave");
    const absent = count("absent");
    const workDays = present + leave + absent;
    return {
      employee_id: e.id,
      nip: e.nip,
      full_name: e.full_name,
      division: e.division_name,
      subdivision: e.subdivision_name,
      work_days: workDays,
      present,
      late,
      leave,
      absent,
      late_minutes: records.reduce((n, r) => n + r.late_minutes, 0),
      attendance_rate: workDays ? present / workDays : 0,
    };
  });
}

/** QUERY.sql #9 - laporan upah berdasarkan presensi (rumus sementara). */
export function payrollReport(db: DB, year: number, month: number, divisionId?: string | null): PayrollReportRow[] {
  return monthRows(db, year, month, divisionId).map(({ employee: e, records }) => {
    const absent = records.filter((r) => r.status === "absent").length;
    const late = records.filter((r) => r.status === "late").length;
    const absentDeduction = Math.round((absent * e.base_salary) / PAYROLL_RULES.work_days_divisor);
    const lateDeduction = late * PAYROLL_RULES.late_penalty;
    return {
      employee_id: e.id,
      nip: e.nip,
      full_name: e.full_name,
      division: e.division_name,
      subdivision: e.subdivision_name,
      base_salary: e.base_salary,
      absent,
      late,
      absent_deduction: absentDeduction,
      late_deduction: lateDeduction,
      net: Math.max(0, e.base_salary - absentDeduction - lateDeduction),
    };
  });
}

export function nextNip(db: DB, year = new Date().getFullYear()) {
  const max = db.employees.reduce((m, e) => Math.max(m, Number(e.nip.slice(-4)) || 0), 0);
  return `PJ${year}${String(max + 1).padStart(4, "0")}`;
}
