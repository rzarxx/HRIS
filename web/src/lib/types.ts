// Kontrak data API /v1. Nama field snake_case mengikuti JSON dari backend Go
// dan kolom di QUERY.sql, sehingga halaman tidak perlu diubah saat API asli siap.

export type Role = "pejuang" | "subdiv" | "kadiv" | "sdm";
export type AttendanceStatus = "present" | "late" | "absent" | "leave" | "review";
export type AnnouncementType = "popup" | "banner";
export type EmployeeStatus = "active" | "inactive";

export interface Division {
  id: string;
  code: string;
  name: string;
  head_employee_id: string | null;
  created_at: string;
}

export interface DivisionWithStats extends Division {
  subdivision_count: number;
  employee_count: number;
  head_name: string | null;
}

export interface Subdivision {
  id: string;
  division_id: string;
  code: string;
  name: string;
  head_employee_id: string | null;
  created_at: string;
}

export interface SubdivisionWithStats extends Subdivision {
  division_name: string;
  employee_count: number;
  head_name: string | null;
}

export interface Employee {
  id: string;
  nip: string;
  full_name: string;
  email: string | null;
  phone: string | null;
  subdivision_id: string;
  position: string | null;
  join_date: string | null; // YYYY-MM-DD
  base_salary: number;
  photo_url: string | null;
  status: EmployeeStatus;
  created_at: string;
  updated_at: string;
  deleted_at: string | null;
}

export interface EmployeeListItem extends Employee {
  subdivision_name: string;
  division_id: string;
  division_name: string;
}

export interface CurrentUser {
  id: string;
  email: string;
  role: Role;
  employee_id: string;
  full_name: string;
  position: string | null;
}

export interface Announcement {
  id: string;
  title: string;
  body: string;
  type: AnnouncementType;
  image_url: string | null;
  division_id: string | null; // null = semua divisi
  starts_at: string;
  ends_at: string | null;
  is_active: boolean;
  created_by: string | null;
  created_at: string;
}

export interface AnnouncementListItem extends Announcement {
  division_name: string | null;
}

export interface OrgChartData {
  summary: {
    employees: number;
    divisions: number;
    subdivisions: number;
    present_today: number;
    late_today: number;
    work_date: string;
  };
  per_subdivision: {
    subdivision_id: string;
    subdivision: string;
    division_id: string;
    division: string;
    employees: number;
  }[];
  per_division: {
    division_id: string;
    division: string;
    subdivisions: number;
    employees: number;
  }[];
}

export interface AttendanceReportRow {
  employee_id: string;
  nip: string;
  full_name: string;
  division: string;
  subdivision: string;
  work_days: number;
  present: number; // hadir (termasuk telat)
  late: number;
  leave: number;
  absent: number;
  late_minutes: number;
  attendance_rate: number; // 0..1
}

export interface PayrollRules {
  work_days_divisor: number;
  late_penalty: number;
  note: string;
}

export interface PayrollReportRow {
  employee_id: string;
  nip: string;
  full_name: string;
  division: string;
  subdivision: string;
  base_salary: number;
  absent: number;
  late: number;
  absent_deduction: number;
  late_deduction: number;
  net: number;
}

export interface Paginated<T> {
  data: T[];
  meta: { page: number; per_page: number; total: number };
}

export interface ListResponse<T> {
  data: T[];
}

export interface ItemResponse<T> {
  data: T;
}
