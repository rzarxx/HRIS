"use client";

import { Select } from "@/components/ui/form";
import { MONTHS, todayWib } from "@/lib/format";
import { useDivisions, type PeriodFilter } from "@/lib/hooks";

export function currentPeriod(offsetMonths = 0): PeriodFilter {
  const [y, m] = todayWib().split("-").map(Number);
  const d = new Date(Date.UTC(y, m - 1 + offsetMonths, 1));
  return { year: d.getUTCFullYear(), month: d.getUTCMonth() + 1 };
}

export function PeriodFilterBar({ value, onChange }: { value: PeriodFilter; onChange: (v: PeriodFilter) => void }) {
  const divisions = useDivisions();
  const thisYear = currentPeriod().year;
  const years = [thisYear - 1, thisYear];
  return (
    <div className="flex flex-wrap gap-3">
      <Select value={value.month} onChange={(e) => onChange({ ...value, month: Number(e.target.value) })} aria-label="Bulan" className="w-auto">
        {MONTHS.map((m, i) => (
          <option key={m} value={i + 1}>
            {m}
          </option>
        ))}
      </Select>
      <Select value={value.year} onChange={(e) => onChange({ ...value, year: Number(e.target.value) })} aria-label="Tahun" className="w-auto">
        {years.map((y) => (
          <option key={y} value={y}>
            {y}
          </option>
        ))}
      </Select>
      <Select
        value={value.division_id ?? ""}
        onChange={(e) => onChange({ ...value, division_id: e.target.value || undefined })}
        aria-label="Divisi"
        className="w-auto min-w-44"
      >
        <option value="">Semua divisi</option>
        {divisions.data?.map((d) => (
          <option key={d.id} value={d.id}>
            {d.name}
          </option>
        ))}
      </Select>
    </div>
  );
}
