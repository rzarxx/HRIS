import Image from "next/image";

export const APP_NAME = "HRIS Yayasan Al Bahjah";

export function Logo({ compact = false }: { compact?: boolean }) {
  return (
    <div className="flex items-center gap-2.5">
      <Image src="/logo-albahjah.png" alt="Logo PUTI Al Bahjah" width={32} height={32} unoptimized className="size-8 shrink-0 object-contain" />
      {!compact && (
        <div className="leading-tight">
          <p className="text-sm font-bold text-slate-900">{APP_NAME}</p>
          <p className="text-[11px] text-slate-500">Web Admin SDM</p>
        </div>
      )}
    </div>
  );
}
