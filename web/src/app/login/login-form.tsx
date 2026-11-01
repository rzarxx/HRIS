"use client";

import { Eye, EyeOff } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/form";
import { api, ApiError } from "@/lib/api-client";
import { loginSchema, validate } from "@/lib/schemas";

const DEMO = process.env.NEXT_PUBLIC_DEMO_MODE === "1";

export function LoginForm() {
  const router = useRouter();
  const next = useSearchParams().get("next");
  const [email, setEmail] = useState(DEMO ? "sdm@pejuang.id" : "");
  const [password, setPassword] = useState("");
  const [show, setShow] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setFormError("");
    const fieldErrors = validate(loginSchema, { email, password });
    setErrors(fieldErrors ?? {});
    if (fieldErrors) return;

    setLoading(true);
    try {
      await api.post("/auth/login", { email, password });
      // Hanya izinkan redirect ke path internal
      router.replace(next && next.startsWith("/") && !next.startsWith("//") ? next : "/");
      router.refresh();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Tidak dapat terhubung ke server.");
      setLoading(false);
    }
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-4">
      {formError && (
        <div role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {formError}
        </div>
      )}
      <Field label="Email" htmlFor="email" error={errors.email}>
        <Input
          id="email"
          type="email"
          autoComplete="username"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          invalid={!!errors.email}
          placeholder="nama@pejuang.id"
        />
      </Field>
      <Field label="Password" htmlFor="password" error={errors.password}>
        <div className="relative">
          <Input
            id="password"
            type={show ? "text" : "password"}
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            invalid={!!errors.password}
            className="pr-10"
          />
          <button
            type="button"
            onClick={() => setShow((s) => !s)}
            aria-label={show ? "Sembunyikan password" : "Tampilkan password"}
            className="absolute inset-y-0 right-0 flex w-10 items-center justify-center text-slate-400 hover:text-slate-600"
          >
            {show ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
          </button>
        </div>
      </Field>
      <Button type="submit" loading={loading} className="w-full">
        Masuk
      </Button>
      {DEMO && (
        <p className="rounded-md bg-slate-50 px-3 py-2 text-xs text-slate-500">
          Mode demo: akun <span className="font-medium text-slate-700">sdm@pejuang.id</span>, password ada di README.
        </p>
      )}
    </form>
  );
}
