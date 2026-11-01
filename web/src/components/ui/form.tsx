import type { InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";

const control =
  "block rounded-md border bg-white px-3 text-sm text-slate-900 placeholder:text-slate-400 focus:outline-none focus:ring-2 disabled:bg-slate-50";
const ok = "border-slate-300 focus:border-primary-600 focus:ring-primary-100";
const bad = "border-red-400 focus:border-red-500 focus:ring-red-100";
// Lebar penuh kecuali className menentukan lebar sendiri (mis. w-auto pada filter)
const width = (cls: string) => (/(^|\s)w-/.test(cls) ? "" : "w-full");

export function Field({
  label,
  htmlFor,
  error,
  hint,
  required,
  children,
  className = "",
}: {
  label: string;
  htmlFor: string;
  error?: string;
  hint?: string;
  required?: boolean;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={className}>
      <label htmlFor={htmlFor} className="mb-1 block text-sm font-medium text-slate-700">
        {label}
        {required && <span className="ml-0.5 text-red-600">*</span>}
      </label>
      {children}
      {error ? (
        <p id={`${htmlFor}-error`} className="mt-1 text-xs text-red-600">
          {error}
        </p>
      ) : hint ? (
        <p className="mt-1 text-xs text-slate-500">{hint}</p>
      ) : null}
    </div>
  );
}

export function Input({ invalid, className = "", ...props }: InputHTMLAttributes<HTMLInputElement> & { invalid?: boolean }) {
  return (
    <input
      aria-invalid={invalid || undefined}
      aria-describedby={invalid && props.id ? `${props.id}-error` : undefined}
      className={`${control} ${width(className)} h-9 ${invalid ? bad : ok} ${className}`}
      {...props}
    />
  );
}

export function Select({ invalid, className = "", ...props }: SelectHTMLAttributes<HTMLSelectElement> & { invalid?: boolean }) {
  return (
    <select
      aria-invalid={invalid || undefined}
      aria-describedby={invalid && props.id ? `${props.id}-error` : undefined}
      className={`${control} ${width(className)} h-9 pr-8 ${invalid ? bad : ok} ${className}`}
      {...props}
    />
  );
}

export function Textarea({ invalid, className = "", ...props }: TextareaHTMLAttributes<HTMLTextAreaElement> & { invalid?: boolean }) {
  return (
    <textarea
      aria-invalid={invalid || undefined}
      aria-describedby={invalid && props.id ? `${props.id}-error` : undefined}
      className={`${control} ${width(className)} py-2 ${invalid ? bad : ok} ${className}`}
      {...props}
    />
  );
}
