import {
  CalendarCheck,
  Database,
  FileText,
  LayoutDashboard,
  Megaphone,
  Settings,
  Smartphone,
  TriangleAlert,
  Wallet,
  type LucideIcon,
} from "lucide-react";

export interface NavChild {
  label: string;
  href: string;
  soon?: boolean;
  permission?: string;
}

export interface NavItem {
  label: string;
  href?: string;
  icon: LucideIcon;
  permission?: string;
  children?: NavChild[];
}

// Urutan menu mengikuti MASTER.md §10.1. `soon` = modul fase berikutnya.
// Permission di sini hanya menyembunyikan menu; yang menegakkan tetap API (§3.3).
export const NAV: NavItem[] = [
  { label: "Dashboard", href: "/", icon: LayoutDashboard, permission: "dashboard:read" },
  {
    label: "Master Data",
    icon: Database,
    children: [
      { label: "Divisi", href: "/master/divisi", permission: "org:write" },
      { label: "Subdiv", href: "/master/subdiv", permission: "org:write" },
      { label: "Pejuang", href: "/master/pejuang", permission: "employee:read" },
      { label: "Lokasi Kantor", href: "/master/lokasi", permission: "location:manage" },
      { label: "Shift", href: "/master/shift", permission: "shift:manage" },
    ],
  },
  {
    label: "Presensi",
    icon: CalendarCheck,
    children: [
      { label: "Rekap", href: "/presensi/rekap", permission: "attendance:report" },
      { label: "Monitor Presensi", href: "/presensi/monitor", permission: "attendance:review" },
      { label: "Review Presensi", href: "/presensi/review", permission: "attendance:review" },
      { label: "Upaya Presensi", href: "/presensi/upaya", permission: "attendance:review" },
    ],
  },
  { label: "Perangkat", href: "/perangkat", icon: Smartphone, permission: "device:review" },
  { label: "Payroll", href: "/payroll", icon: Wallet, permission: "payroll:manage" },
  {
    label: "Pengajuan",
    icon: FileText,
    children: [
      { label: "Cuti & Izin", href: "/pengajuan/cuti", soon: true },
      { label: "Pinjaman", href: "/pengajuan/kasbon", soon: true },
      { label: "Mutasi", href: "/pengajuan/mutasi", soon: true },
    ],
  },
  { label: "Surat Peringatan", href: "/surat-peringatan", icon: TriangleAlert, permission: "warning:manage" },
  { label: "Pengumuman", href: "/pengumuman", icon: Megaphone, permission: "announcement:manage" },
  { label: "Pengaturan", href: "/pengaturan", icon: Settings },
];

/** Menyaring menu sesuai permissions dari GET /v1/me. */
export function visibleNav(permissions: string[]): NavItem[] {
  const allowed = (key?: string) => !key || permissions.includes(key);
  return NAV.flatMap((item) => {
    if (!allowed(item.permission)) return [];
    if (!item.children) return [item];
    const children = item.children.filter((c) => allowed(c.permission));
    return children.length ? [{ ...item, children }] : [];
  });
}
