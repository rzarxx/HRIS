import {
  CalendarCheck,
  Database,
  FileText,
  LayoutDashboard,
  Megaphone,
  Settings,
  TriangleAlert,
  Wallet,
  type LucideIcon,
} from "lucide-react";

export interface NavItem {
  label: string;
  href?: string;
  icon: LucideIcon;
  children?: { label: string; href: string; soon?: boolean }[];
}

// Urutan menu mengikuti DESIGN.md §7. `soon` = modul fase berikutnya.
export const NAV: NavItem[] = [
  { label: "Dashboard", href: "/", icon: LayoutDashboard },
  {
    label: "Master Data",
    icon: Database,
    children: [
      { label: "Divisi", href: "/master/divisi" },
      { label: "Subdiv", href: "/master/subdiv" },
      { label: "Pejuang", href: "/master/pejuang" },
      { label: "Lokasi Kantor", href: "/master/lokasi", soon: true },
      { label: "Shift", href: "/master/shift", soon: true },
    ],
  },
  {
    label: "Presensi",
    icon: CalendarCheck,
    children: [
      { label: "Rekap", href: "/presensi/rekap" },
      { label: "Review Absen", href: "/presensi/review", soon: true },
    ],
  },
  { label: "Payroll", href: "/payroll", icon: Wallet },
  {
    label: "Pengajuan",
    icon: FileText,
    children: [
      { label: "Cuti & Izin", href: "/pengajuan/cuti", soon: true },
      { label: "Kasbon", href: "/pengajuan/kasbon", soon: true },
      { label: "Mutasi", href: "/pengajuan/mutasi", soon: true },
    ],
  },
  { label: "Surat Peringatan", href: "/surat-peringatan", icon: TriangleAlert },
  { label: "Pengumuman", href: "/pengumuman", icon: Megaphone },
  { label: "Pengaturan", href: "/pengaturan", icon: Settings },
];
