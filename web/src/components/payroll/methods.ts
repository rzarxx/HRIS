import type { PayrollCalcMethod } from "@/lib/types";

/** Isian yang dipakai tiap cara hitung (D66). Isian lain dikosongkan server. */
export interface MethodInfo {
  label: string;
  hint: string;
  rate?: "required" | "optional";
  rateLabel?: string;
  percent?: boolean;
  divisor?: boolean;
}

export const METHODS: Record<PayrollCalcMethod, MethodInfo> = {
  fixed: { label: "Nominal tetap", hint: "Nominal sama untuk semua Pejuang.", rate: "required", rateLabel: "Nominal" },
  per_employee: { label: "Nominal per Pejuang", hint: "Nominal diisi di form tiap Pejuang, bagian Komponen ubar." },
  per_present_day: { label: "Per hari hadir", hint: "Nominal dikali jumlah hari hadir.", rate: "required", rateLabel: "Nominal per hari" },
  per_late: { label: "Per kali telat", hint: "Nominal dikali jumlah kali telat.", rate: "required", rateLabel: "Nominal per kali" },
  per_late_minute: { label: "Per menit telat", hint: "Nominal dikali total menit telat.", rate: "required", rateLabel: "Nominal per menit" },
  per_absent_day: {
    label: "Per hari alpha",
    hint: "Nominal dikali hari alpha. Bila nominal kosong: ubar pokok dibagi pembagi hari, dikali hari alpha.",
    rate: "optional",
    rateLabel: "Nominal per hari",
    divisor: true,
  },
  per_unpaid_leave_day: {
    label: "Per hari cuti tidak dibayar",
    hint: "Seperti per hari alpha, untuk cuti yang tidak dibayar.",
    rate: "optional",
    rateLabel: "Nominal per hari",
    divisor: true,
  },
  percent_of_base: { label: "Persen dari ubar pokok", hint: "Contoh: BPJS.", percent: true },
  percent_of_gross: { label: "Persen dari total pendapatan", hint: "Contoh: PPh 21 sederhana. Dihitung setelah semua pendapatan.", percent: true },
  per_unit: { label: "Per satuan (isian per periode)", hint: "Nominal dikali kuantitas yang diisi SDM tiap periode. Contoh: jam lembur.", rate: "required", rateLabel: "Nominal per satuan" },
  manual: { label: "Isian manual per periode", hint: "Nominal diisi SDM per Pejuang tiap periode." },
};

export const METHOD_ORDER = Object.keys(METHODS) as PayrollCalcMethod[];
