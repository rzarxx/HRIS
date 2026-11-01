import { ShieldCheck } from "lucide-react";

export function Logo({ compact = false }: { compact?: boolean }) {
  return (
    <div className="flex items-center gap-2.5">
      <div className="flex size-8 items-center justify-center rounded-lg bg-primary-600 text-white">
        <ShieldCheck className="size-5" aria-hidden />
      </div>
      {!compact && (
        <div className="leading-tight">
          <p className="text-sm font-bold text-slate-900">HRIS Pejuang</p>
          <p className="text-[11px] text-slate-500">Web Admin SDM</p>
        </div>
      )}
    </div>
  );
}
