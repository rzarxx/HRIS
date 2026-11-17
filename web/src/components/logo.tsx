import Image from "next/image";

export const APP_NAME = "HRIS Al Bahjah";

/** Rasio logo Al Bahjah (lebar : tinggi) setelah tepi kosongnya dipotong. */
const LOGO_RATIO = 915 / 586;

/**
 * Logo Al Bahjah beserta nama aplikasi. `size="lg"` dipakai di halaman masuk agar tulisan pada logo
 * terbaca; sidebar memakai ukuran bawaan.
 */
export function Logo({ compact = false, size = "md" }: { compact?: boolean; size?: "md" | "lg" }) {
  if (size === "lg") {
    const height = 96;
    return (
      <div className="flex flex-col items-center gap-2 text-center">
        <Image src="/logo-albahjah.png" alt="Logo Al Bahjah" width={Math.round(height * LOGO_RATIO)} height={height} unoptimized priority className="h-24 w-auto" />
        <div className="leading-tight">
          <p className="text-base font-bold text-slate-900">{APP_NAME}</p>
          <p className="text-xs text-slate-500">Web Admin SDM</p>
        </div>
      </div>
    );
  }
  return (
    <div className="flex items-center gap-2.5">
      <Image src="/logo-albahjah.png" alt="Logo Al Bahjah" width={50} height={32} unoptimized className="h-8 w-[50px] shrink-0 object-contain" />
      {!compact && (
        <div className="leading-tight">
          <p className="text-sm font-bold whitespace-nowrap text-slate-900">{APP_NAME}</p>
          <p className="text-[11px] text-slate-500">Web Admin SDM</p>
        </div>
      )}
    </div>
  );
}
