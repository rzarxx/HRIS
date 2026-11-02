"use client";

import { Building2, CalendarCheck, Network, Users } from "lucide-react";
import Link from "next/link";
import { AnnouncementStatusBadge } from "@/components/announcements/status";
import { EmployeesPerSubdivChart, SubdivsPerDivisionChart } from "@/components/dashboard/charts";
import { Card, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { dateLong, dateTime, number, percent } from "@/lib/format";
import { useAnnouncements, useDashboardSummary, useMe, useOrgChart } from "@/lib/hooks";

function Stat({ label, value, sub, icon: Icon }: { label: string; value: string; sub?: string; icon: typeof Users }) {
  return (
    <div className="rounded-[var(--radius-card)] border border-slate-200 bg-white p-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-slate-500">{label}</p>
        <Icon className="size-4 text-slate-400" aria-hidden />
      </div>
      <p className="mt-2 text-2xl font-semibold text-slate-900 tabular-nums">{value}</p>
      {sub && <p className="mt-0.5 text-xs text-slate-500">{sub}</p>}
    </div>
  );
}

export default function DashboardPage() {
  const { data: me } = useMe();
  const summary = useDashboardSummary();
  const chart = useOrgChart();
  const announcements = useAnnouncements();

  if (summary.isPending || chart.isPending) return <Loading />;
  if (summary.isError) return <ErrorState message={summary.error.message} onRetry={() => summary.refetch()} />;
  if (chart.isError) return <ErrorState message={chart.error.message} onRetry={() => chart.refetch()} />;

  const s = summary.data;
  return (
    <>
      <PageHeader
        title={`Selamat datang${me ? `, ${me.full_name.split(" ")[0]}` : ""}`}
        description={`Ringkasan organisasi per ${dateLong(s.work_date)}`}
      />

      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Stat label="Total Pejuang aktif" value={number(s.employee_count)} icon={Users} />
        <Stat label="Divisi" value={number(s.division_count)} icon={Building2} />
        <Stat label="Subdiv" value={number(s.subdivision_count)} icon={Network} />
        <Stat
          label="Hadir hari ini"
          value={number(s.present_today)}
          sub={`${s.employee_count ? percent(s.present_today / s.employee_count) : "0%"} · ${number(s.late_today)} terlambat`}
          icon={CalendarCheck}
        />
      </div>

      <div className="mt-4 grid gap-4 xl:grid-cols-5">
        <div className="xl:col-span-3">
          <EmployeesPerSubdivChart data={chart.data} />
        </div>
        <div className="flex flex-col gap-4 xl:col-span-2">
          <SubdivsPerDivisionChart data={chart.data} />
          <Card
            title="Pengumuman terbaru"
            actions={
              <Link href="/pengumuman" className="text-xs font-medium text-primary-600 hover:underline">
                Lihat semua
              </Link>
            }
            bodyClassName="divide-y divide-slate-100"
          >
            {announcements.data?.slice(0, 4).map((a) => (
              <div key={a.id} className="px-5 py-3">
                <div className="flex items-start justify-between gap-2">
                  <p className="text-sm font-medium text-slate-800">{a.title}</p>
                  <AnnouncementStatusBadge a={a} />
                </div>
                <p className="mt-0.5 text-xs text-slate-500">
                  {a.type === "popup" ? "Pop-up" : "Banner"} · {a.division_name ?? "Semua divisi"} · {dateTime(a.starts_at)}
                </p>
              </div>
            )) ?? <Loading />}
            {announcements.data?.length === 0 && <p className="px-5 py-6 text-sm text-slate-500">Belum ada pengumuman.</p>}
          </Card>
        </div>
      </div>
    </>
  );
}
