// Alias nama pendek untuk tipe hasil generate. Jangan menulis tipe API manual di sini:
// ubah backend/api/openapi.yaml lalu jalankan `npm run gen:api` (MASTER.md §7.7).
import type { components } from "@/api/schema";

type Schemas = components["schemas"];

export type Role = Schemas["Role"];
export type AnnouncementType = Schemas["AnnouncementType"];
export type EmployeeStatus = Schemas["EmployeeStatus"];
export type Money = Schemas["Money"];
export type ErrorCode = Schemas["ErrorCode"];
export type Permission = Schemas["Permission"];

export type DivisionWithStats = Schemas["Division"];
export type SubdivisionWithStats = Schemas["Subdivision"];
export type EmployeeListItem = Schemas["Employee"];
export type EmployeeCreated = Schemas["EmployeeCreated"];
export type Activation = Schemas["Activation"];
export type CurrentUser = Schemas["Me"];
export type AnnouncementListItem = Schemas["Announcement"];
export type DashboardSummary = Schemas["DashboardSummary"];
export type OrgChartData = Schemas["OrgChart"];
export type AttendanceReportRow = Schemas["AttendanceReportRow"];
export type PayrollReportRow = Schemas["PayrollPreviewRow"];
export type PayrollRules = Schemas["PayrollRules"];
export type Session = Schemas["Session"];
export type OfficeLocation = Schemas["OfficeLocation"];
export type Shift = Schemas["Shift"];
export type Holiday = Schemas["Holiday"];
export type AdminDevice = Schemas["AdminDevice"];
export type DeviceStatus = Schemas["DeviceStatus"];
export type MfaSetup = Schemas["MfaSetup"];
export type MfaEnabled = Schemas["MfaEnabled"];

export interface Paginated<T> {
  data: T[];
  meta: Schemas["PageMeta"];
}

export interface ListResponse<T> {
  data: T[];
}

export interface ItemResponse<T> {
  data: T;
}
