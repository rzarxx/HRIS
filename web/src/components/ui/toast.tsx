"use client";

import { CheckCircle2, XCircle } from "lucide-react";
import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";

type Toast = { id: number; kind: "success" | "error"; message: string };
const ToastContext = createContext<(kind: Toast["kind"], message: string) => void>(() => {});

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const layer = useRef<HTMLDivElement>(null);
  const push = useCallback((kind: Toast["kind"], message: string) => {
    const id = Date.now() + Math.random();
    setToasts((t) => [...t, { id, kind, message }]);
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 3500);
  }, []);

  // Dialog terbuka menutupi semua z-index, jadi toast naik ke top layer. Dibuka ulang agar berada di atas dialog.
  useEffect(() => {
    const el = layer.current;
    if (!el || !("showPopover" in el)) return;
    if (el.matches(":popover-open")) el.hidePopover();
    if (toasts.length > 0) el.showPopover();
  }, [toasts]);

  return (
    <ToastContext.Provider value={push}>
      {children}
      <div
        ref={layer}
        popover="manual"
        className="pointer-events-none fixed inset-auto right-4 bottom-4 m-0 flex-col gap-2 overflow-visible border-0 bg-transparent p-0 [&:popover-open]:flex"
        role="status"
        aria-live="polite"
      >
        {toasts.map((t) => (
          <div
            key={t.id}
            className="pointer-events-auto flex max-w-sm items-start gap-2 rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm shadow-lg"
          >
            {t.kind === "success" ? (
              <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-600" aria-hidden />
            ) : (
              <XCircle className="mt-0.5 size-4 shrink-0 text-red-600" aria-hidden />
            )}
            <span className="text-slate-700">{t.message}</span>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const push = useContext(ToastContext);
  return {
    success: (m: string) => push("success", m),
    error: (m: string) => push("error", m),
  };
}
