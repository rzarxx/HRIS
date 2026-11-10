import { CalendarCheck, FileText, Home, Megaphone, User, Wallet, X } from "lucide-react";
import type { AnnouncementType } from "@/lib/types";

/** Pratinjau tampilan pengumuman di aplikasi HP Pejuang (DESIGN.md §8). */
export function PhonePreview({ title, body, type }: { title: string; body: string; type: AnnouncementType }) {
  const t = title.trim() || "Judul pengumuman";
  const b = body.trim() || "Isi pengumuman akan tampil di sini.";
  return (
    <div className="mx-auto w-[260px] rounded-[28px] border-[6px] border-slate-800 bg-slate-800 shadow-lg" aria-label="Pratinjau di aplikasi HP">
      <div className="relative h-[480px] overflow-hidden rounded-[22px] bg-slate-50">
        <div className="flex items-center justify-between px-4 pt-2 text-[10px] text-slate-500">
          <span>08.02</span>
          <span>4G ▮▮▮</span>
        </div>
        <div className="px-4 pt-3">
          <p className="text-[11px] text-slate-500">Selamat pagi,</p>
          <p className="text-sm font-semibold text-slate-900">Pejuang</p>
        </div>

        {type === "banner" && (
          <div className="mx-3 mt-3 rounded-lg bg-primary-600 p-3 text-white">
            <div className="flex items-center gap-1.5 text-[10px] font-medium opacity-90">
              <Megaphone className="size-3" aria-hidden /> Pengumuman
            </div>
            <p className="mt-1 text-xs font-semibold">{t}</p>
            <p className="mt-0.5 line-clamp-3 text-[11px] opacity-90">{b}</p>
          </div>
        )}

        <div className="mx-3 mt-3 rounded-lg border border-slate-200 bg-white p-3 text-center">
          <p className="text-[10px] text-slate-500">Shift Reguler · 08.00 - 17.00</p>
          <div className="mx-auto mt-2 flex size-16 items-center justify-center rounded-full bg-primary-600 text-[11px] font-semibold text-white">
            Presensi
          </div>
          <p className="mt-2 text-[10px] text-emerald-600">Anda berada di area kantor</p>
        </div>

        <div className="absolute inset-x-0 bottom-0 flex justify-around border-t border-slate-200 bg-white py-2 text-[9px] text-slate-500">
          {[
            [Home, "Beranda"],
            [CalendarCheck, "Presensi"],
            [FileText, "Pengajuan"],
            [Wallet, "Slip"],
            [User, "Profil"],
          ].map(([Icon, label], i) => {
            const I = Icon as typeof Home;
            return (
              <span key={label as string} className={`flex flex-col items-center gap-0.5 ${i === 0 ? "text-primary-600" : ""}`}>
                <I className="size-3.5" aria-hidden />
                {label as string}
              </span>
            );
          })}
        </div>

        {type === "popup" && (
          <div className="absolute inset-0 flex items-center justify-center bg-slate-900/50 p-4">
            <div className="w-full rounded-xl bg-white p-4 shadow-xl">
              <div className="flex items-start justify-between gap-2">
                <div className="flex size-8 items-center justify-center rounded-full bg-primary-50">
                  <Megaphone className="size-4 text-primary-600" aria-hidden />
                </div>
                <X className="size-4 text-slate-400" aria-hidden />
              </div>
              <p className="mt-2 text-sm font-semibold text-slate-900">{t}</p>
              <p className="mt-1 max-h-40 overflow-hidden text-[11px] text-slate-600">{b}</p>
              <div className="mt-3 rounded-md bg-primary-600 py-1.5 text-center text-xs font-medium text-white">Saya mengerti</div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
