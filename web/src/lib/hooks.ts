"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "./api-client";
import type {
  AdminDevice,
  AnnouncementListItem,
  AnnouncementReader,
  Branding,
  ApprovalFlow,
  ApprovalItem,
  AttendanceAttempt,
  AttendanceMonitorItem,
  AttendanceReportRow,
  AttendanceReviewItem,
  CurrentUser,
  DashboardSummary,
  DivisionWithStats,
  EmployeeListItem,
  Holiday,
  ItemResponse,
  LeaveRequest,
  LeaveType,
  ListResponse,
  OfficeLocation,
  OrgChartData,
  Paginated,
  PayrollReportRow,
  PayrollRules,
  Shift,
  WorkArrangement,
  SubdivisionWithStats,
} from "./types";
import { useToast } from "@/components/ui/toast";

export const useMe = () =>
  useQuery({
    queryKey: ["me"],
    queryFn: () => api.get<ItemResponse<CurrentUser>>("/me").then((r) => r.data),
    staleTime: Infinity,
    retry: false,
  });

export const useDashboardSummary = () =>
  useQuery({
    queryKey: ["dashboard-summary"],
    queryFn: () => api.get<ItemResponse<DashboardSummary>>("/admin/dashboard/summary").then((r) => r.data),
  });

export const useOrgChart = () =>
  useQuery({
    queryKey: ["org-chart"],
    queryFn: () => api.get<ItemResponse<OrgChartData>>("/admin/dashboard/org-chart").then((r) => r.data),
  });

export const useDivisions = () =>
  useQuery({
    queryKey: ["divisions"],
    queryFn: () => api.get<ListResponse<DivisionWithStats>>("/admin/divisions").then((r) => r.data),
  });

export const useSubdivisions = () =>
  useQuery({
    queryKey: ["subdivisions"],
    queryFn: () => api.get<ListResponse<SubdivisionWithStats>>("/admin/subdivisions").then((r) => r.data),
  });

export const useOfficeLocations = () =>
  useQuery({
    queryKey: ["office-locations"],
    queryFn: () => api.get<ListResponse<OfficeLocation>>("/admin/office-locations").then((r) => r.data),
  });

export const useEmployeeLocations = (employeeId: string) =>
  useQuery({
    queryKey: ["employee-locations", employeeId],
    queryFn: () => api.get<ListResponse<OfficeLocation>>(`/admin/employees/${employeeId}/locations`).then((r) => r.data),
  });

export const useWorkArrangements = (employeeId: string) =>
  useQuery({
    queryKey: ["work-arrangements", employeeId],
    queryFn: () => api.get<ListResponse<WorkArrangement>>(`/admin/employees/${employeeId}/work-arrangements`).then((r) => r.data),
  });

// Diperbarui tiap 60 detik selama yang dilihat adalah hari ini.
export const useAttendanceMonitor = (f: { date: string; division_id: string }, live: boolean) =>
  useQuery({
    queryKey: ["attendance-monitor", f],
    queryFn: () => api.get<ListResponse<AttendanceMonitorItem>>("/admin/attendance/monitor", { ...f }).then((r) => r.data),
    placeholderData: keepPreviousData,
    refetchInterval: live ? 60_000 : false,
  });

export const useAttendanceReview = (f: { work_mode: string; page: number; per_page: number }) =>
  useQuery({
    queryKey: ["attendance-review", f],
    queryFn: () => api.get<Paginated<AttendanceReviewItem>>("/admin/attendance/review", { ...f }),
    placeholderData: keepPreviousData,
  });

export const useAttendanceAttempts = (f: { date: string; outcome: string; page: number; per_page: number }) =>
  useQuery({
    queryKey: ["attendance-attempts", f],
    queryFn: () => api.get<Paginated<AttendanceAttempt>>("/admin/attendance/attempts", { ...f }),
    placeholderData: keepPreviousData,
  });

export const useShifts = () =>
  useQuery({
    queryKey: ["shifts"],
    queryFn: () => api.get<ListResponse<Shift>>("/admin/shifts").then((r) => r.data),
  });

export const useHolidays = (year: number) =>
  useQuery({
    queryKey: ["holidays", year],
    queryFn: () => api.get<ListResponse<Holiday>>("/admin/holidays", { year }).then((r) => r.data),
  });

export const useAdminDevices = (f: { status: string; page: number; per_page: number }) =>
  useQuery({
    queryKey: ["admin-devices", f],
    queryFn: () => api.get<Paginated<AdminDevice>>("/admin/devices", { ...f }),
    placeholderData: keepPreviousData,
  });

export interface EmployeeFilter {
  q?: string;
  division_id?: string;
  subdivision_id?: string;
  status?: string;
  page: number;
  per_page: number;
}

export const useEmployees = (filter: EmployeeFilter) =>
  useQuery({
    queryKey: ["employees", filter],
    queryFn: () => api.get<Paginated<EmployeeListItem>>("/admin/employees", { ...filter }),
    placeholderData: keepPreviousData,
  });

/** Pejuang aktif untuk pilihan kepala divisi/subdiv (maksimum per_page API = 100). */
export const useAllEmployees = () =>
  useQuery({
    queryKey: ["employees", "all"],
    queryFn: () => api.get<Paginated<EmployeeListItem>>("/admin/employees", { per_page: 100, status: "active" }).then((r) => r.data),
  });

export const useAnnouncements = () =>
  useQuery({
    queryKey: ["announcements"],
    queryFn: () => api.get<Paginated<AnnouncementListItem>>("/admin/announcements", { per_page: 100 }).then((r) => r.data),
  });

/** Teks bawaan bila footer belum termuat; tahun mengikuti jam perangkat. */
export const defaultFooter = () => `© ${new Date().getFullYear()} PUTI (Pusat Teknologi Informasi) Al Bahjah`;

/** Footer dan merek; tanpa login, dipakai juga di halaman masuk. */
export const useBranding = () =>
  useQuery({
    queryKey: ["branding"],
    queryFn: () => api.get<ItemResponse<Branding>>("/branding").then((r) => r.data),
    staleTime: 60 * 60 * 1000,
    retry: 1,
  });

/** Status baca tiap Pejuang sasaran satu pengumuman; dimuat saat dialog rekap dibuka. */
export const useAnnouncementReaders = (id: string | null) =>
  useQuery({
    queryKey: ["announcements", id, "reads"],
    queryFn: () => api.get<ListResponse<AnnouncementReader>>(`/admin/announcements/${id}/reads`).then((r) => r.data),
    enabled: id !== null,
  });

export interface LeaveFilter {
  from: string;
  to: string;
}

/** Semua pengajuan cuti/izin dalam scope untuk rentang tanggal; status disaring di klien agar tab instan. */
export const useAdminLeaves = (f: LeaveFilter) =>
  useQuery({
    queryKey: ["admin-leaves", f],
    queryFn: () => api.get<ListResponse<LeaveRequest>>("/admin/leaves", { ...f }).then((r) => r.data),
    placeholderData: keepPreviousData,
  });

export const useAdminLeaveTypes = () =>
  useQuery({
    queryKey: ["admin-leave-types"],
    queryFn: () => api.get<ListResponse<LeaveType>>("/admin/leave-types").then((r) => r.data),
  });

export const useApprovalFlow = (kind: ApprovalFlow["request_type"]) =>
  useQuery({
    queryKey: ["approval-flow", kind],
    queryFn: () => api.get<ItemResponse<ApprovalFlow>>(`/admin/approval-flows/${kind}`).then((r) => r.data),
  });

/** Kotak masuk persetujuan; diperbarui tiap menit agar lencana menu tetap segar. */
export const usePendingApprovals = (enabled = true) =>
  useQuery({
    queryKey: ["approvals-pending"],
    queryFn: () => api.get<ListResponse<ApprovalItem>>("/approvals/pending").then((r) => r.data),
    enabled,
    refetchInterval: 60_000,
  });

export const useApprovalDetail = (id: string) =>
  useQuery({
    queryKey: ["approval", id],
    queryFn: () => api.get<ItemResponse<ApprovalItem>>(`/approvals/${id}`).then((r) => r.data),
  });

export interface PeriodFilter {
  year: number;
  month: number;
  division_id?: string;
}

export type AttendanceReportFilter = PeriodFilter & { work_mode?: "onsite" | "wfa" | "field" };

export const useAttendanceReport = (f: AttendanceReportFilter) =>
  useQuery({
    queryKey: ["report-attendance", f],
    queryFn: () => api.get<ListResponse<AttendanceReportRow>>("/admin/reports/attendance", { ...f }).then((r) => r.data),
    placeholderData: keepPreviousData,
  });

export const usePayrollReport = (f: PeriodFilter) =>
  useQuery({
    queryKey: ["report-payroll", f],
    queryFn: () => api.get<ListResponse<PayrollReportRow> & { meta: { rules: PayrollRules } }>("/admin/reports/payroll-preview", { ...f }),
    placeholderData: keepPreviousData,
  });

/**
 * Mutasi data: setelah sukses semua query di-refresh (grafik, tabel, laporan ikut berubah).
 * Error validasi per field dikembalikan lewat onFieldErrors.
 */
export function useSave<TVars, TRes = unknown>(
  fn: (vars: TVars) => Promise<TRes>,
  opts: { success?: string; onDone?: (res: TRes) => void; onFieldErrors?: (f: Record<string, string>) => void } = {},
) {
  const qc = useQueryClient();
  const toast = useToast();
  return useMutation({
    mutationFn: fn,
    onSuccess: async (res) => {
      await qc.invalidateQueries({ predicate: (q) => q.queryKey[0] !== "me" });
      if (opts.success) toast.success(opts.success);
      opts.onDone?.(res);
    },
    onError: (e) => {
      if (e instanceof ApiError && e.fields && opts.onFieldErrors) opts.onFieldErrors(e.fields);
      toast.error(e instanceof ApiError ? e.message : "Gagal menyimpan, periksa koneksi.");
    },
  });
}
