"use client";

import { Eye, EyeOff } from "lucide-react";
import { useEffect, useRef, useState, type InputHTMLAttributes } from "react";
import { Input } from "@/components/ui/form";

/**
 * Kolom kata sandi dengan tombol mata untuk mengintip isinya. Isian kembali tersembunyi saat form
 * dikirim, agar kata sandi tidak tertinggal terlihat di layar.
 */
export function PasswordInput({
  invalid,
  className = "",
  ...props
}: Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & { invalid?: boolean }) {
  const [show, setShow] = useState(false);
  const wrap = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const form = wrap.current?.closest("form");
    if (!form) return;
    const hide = () => setShow(false);
    form.addEventListener("submit", hide);
    return () => form.removeEventListener("submit", hide);
  }, []);

  return (
    <div ref={wrap} className="relative">
      <Input
        {...props}
        type={show ? "text" : "password"}
        invalid={invalid}
        autoCapitalize="none"
        spellCheck={false}
        className={`pr-10 ${className}`}
      />
      <button
        type="button"
        onClick={() => setShow((s) => !s)}
        aria-label={show ? "Sembunyikan kata sandi" : "Tampilkan kata sandi"}
        aria-pressed={show}
        aria-controls={props.id}
        className="absolute inset-y-0 right-0 flex w-10 items-center justify-center rounded-r-md text-slate-400 hover:text-slate-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-100"
      >
        {show ? <EyeOff className="size-4" aria-hidden /> : <Eye className="size-4" aria-hidden />}
      </button>
    </div>
  );
}
