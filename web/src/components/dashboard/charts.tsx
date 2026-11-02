"use client";

import { useState } from "react";
import { Bar, BarChart, CartesianGrid, Cell, LabelList, ResponsiveContainer, Tooltip, XAxis, YAxis, type TooltipProps } from "recharts";
import { Card } from "@/components/ui/layout";
import { Table, Td, Th } from "@/components/ui/table";
import { number } from "@/lib/format";
import type { OrgChartData } from "@/lib/types";

// Slot kategori dataviz (urutan tetap). Warna mengikuti divisi, bukan peringkat.
const SERIES = ["#2a78d6", "#eb6834", "#1baf7a", "#eda100", "#e87ba4", "#008300", "#6250d6", "#e34948"];
const AXIS = { fontSize: 12, fill: "#64748b" };

function divisionColors(divisions: { division: string }[]) {
  return new Map(divisions.map((d, i) => [d.division, SERIES[i % SERIES.length]]));
}

function ViewToggle({ table, onChange }: { table: boolean; onChange: (t: boolean) => void }) {
  return (
    <div className="flex rounded-md border border-slate-200 p-0.5 text-xs" role="group" aria-label="Tampilan">
      {[
        ["Grafik", false],
        ["Tabel", true],
      ].map(([label, value]) => (
        <button
          key={String(label)}
          type="button"
          aria-pressed={table === value}
          onClick={() => onChange(value as boolean)}
          className={`rounded px-2 py-0.5 ${table === value ? "bg-slate-100 font-medium text-slate-800" : "text-slate-500"}`}
        >
          {label}
        </button>
      ))}
    </div>
  );
}

function TooltipBox({ title, rows }: { title: string; rows: [string, string][] }) {
  return (
    <div className="rounded-md border border-slate-200 bg-white px-3 py-2 text-xs shadow-md">
      <p className="mb-1 font-semibold text-slate-800">{title}</p>
      {rows.map(([k, v]) => (
        <p key={k} className="flex justify-between gap-4 text-slate-500">
          {k} <span className="font-medium text-slate-800">{v}</span>
        </p>
      ))}
    </div>
  );
}

type SubRow = OrgChartData["employees_per_subdivision"][number];

export function EmployeesPerSubdivChart({ data }: { data: OrgChartData }) {
  const [table, setTable] = useState(false);
  const colors = divisionColors(data.subdivisions_per_division);
  const rows = data.employees_per_subdivision;
  const height = Math.max(240, rows.length * 30 + 40);

  return (
    <Card title="Pejuang per subdiv" actions={<ViewToggle table={table} onChange={setTable} />}>
      <ul className="mb-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-600" aria-label="Legenda divisi">
        {data.subdivisions_per_division.map((d) => (
          <li key={d.division} className="flex items-center gap-1.5">
            <span className="size-2.5 rounded-sm" style={{ background: colors.get(d.division) }} aria-hidden />
            {d.division}
          </li>
        ))}
      </ul>
      {table ? (
        <Table>
          <thead>
            <tr>
              <Th>Divisi</Th>
              <Th>Subdiv</Th>
              <Th align="right">Pejuang</Th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={`${r.division}|${r.subdivision}`}>
                <Td>{r.division}</Td>
                <Td>{r.subdivision}</Td>
                <Td align="right">{number(r.employee_count)}</Td>
              </tr>
            ))}
          </tbody>
        </Table>
      ) : (
        <div style={{ height }} role="img" aria-label="Grafik batang jumlah Pejuang per subdiv">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={rows} layout="vertical" margin={{ top: 0, right: 32, bottom: 0, left: 8 }} barCategoryGap={6}>
              <CartesianGrid horizontal={false} stroke="#e2e8f0" strokeDasharray="3 3" />
              <XAxis type="number" allowDecimals={false} tick={AXIS} axisLine={false} tickLine={false} />
              <YAxis type="category" dataKey="subdivision" width={110} tick={AXIS} axisLine={false} tickLine={false} />
              <Tooltip
                cursor={{ fill: "#f1f5f9" }}
                content={({ active, payload }: TooltipProps<number, string>) => {
                  const r = payload?.[0]?.payload as SubRow | undefined;
                  if (!active || !r) return null;
                  return <TooltipBox title={r.subdivision} rows={[["Divisi", r.division], ["Pejuang", number(r.employee_count)]]} />;
                }}
              />
              <Bar dataKey="employee_count" radius={[0, 4, 4, 0]} maxBarSize={18} isAnimationActive={false}>
                {rows.map((r) => (
                  <Cell key={`${r.division}|${r.subdivision}`} fill={colors.get(r.division)} stroke="#fff" strokeWidth={2} />
                ))}
                <LabelList dataKey="employee_count" position="right" fontSize={12} fill="#334155" />
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </Card>
  );
}

type DivRow = OrgChartData["subdivisions_per_division"][number];

export function SubdivsPerDivisionChart({ data }: { data: OrgChartData }) {
  const [table, setTable] = useState(false);
  const rows = data.subdivisions_per_division;
  const employeesOf = (division: string) =>
    data.employees_per_subdivision.filter((s) => s.division === division).reduce((n, s) => n + s.employee_count, 0);

  return (
    <Card title="Subdiv per divisi" actions={<ViewToggle table={table} onChange={setTable} />}>
      {table ? (
        <Table>
          <thead>
            <tr>
              <Th>Divisi</Th>
              <Th align="right">Subdiv</Th>
              <Th align="right">Pejuang</Th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.division}>
                <Td>{r.division}</Td>
                <Td align="right">{number(r.subdivision_count)}</Td>
                <Td align="right">{number(employeesOf(r.division))}</Td>
              </tr>
            ))}
          </tbody>
        </Table>
      ) : (
        <div className="h-[260px]" role="img" aria-label="Grafik batang jumlah subdiv per divisi">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={rows} margin={{ top: 20, right: 8, bottom: 0, left: -16 }}>
              <CartesianGrid vertical={false} stroke="#e2e8f0" strokeDasharray="3 3" />
              <XAxis
                dataKey="division"
                tick={AXIS}
                axisLine={false}
                tickLine={false}
                interval={0}
                tickFormatter={(v: string) => (v.length > 14 ? `${v.slice(0, 13)}…` : v)}
              />
              <YAxis allowDecimals={false} tick={AXIS} axisLine={false} tickLine={false} />
              <Tooltip
                cursor={{ fill: "#f1f5f9" }}
                content={({ active, payload }: TooltipProps<number, string>) => {
                  const r = payload?.[0]?.payload as DivRow | undefined;
                  if (!active || !r) return null;
                  return <TooltipBox title={r.division} rows={[["Subdiv", number(r.subdivision_count)], ["Pejuang", number(employeesOf(r.division))]]} />;
                }}
              />
              <Bar
                dataKey="subdivision_count"
                fill={SERIES[0]}
                radius={[4, 4, 0, 0]}
                maxBarSize={40}
                isAnimationActive={false}
                label={{ position: "top", fontSize: 12, fill: "#334155" }}
              />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </Card>
  );
}
