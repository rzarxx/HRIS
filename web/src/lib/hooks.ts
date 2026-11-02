"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "./api-client";
import type {
  AnnouncementListItem,
  AttendanceReportRow,
  CurrentUser,
  DashboardSummary,
  DivisionWithStats,
  EmployeeListItem,
  ItemResponse,
  ListResponse,
  OrgChartData,
  Paginated,
  PayrollReportRow,
  PayrollRules,
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

export interface PeriodFilter {
  year: number;
  month: number;
  division_id?: string;
}

export const useAttendanceReport = (f: PeriodFilter) =>
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
