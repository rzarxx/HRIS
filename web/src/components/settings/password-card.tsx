"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/form";
import { PasswordInput } from "@/components/ui/password-input";
import { Card } from "@/components/ui/layout";
import { api } from "@/lib/api-client";
import { useSave } from "@/lib/hooks";

const EMPTY = { current_password: "", new_password: "", confirm: "" };

export function PasswordCard() {
  const [form, setForm] = useState(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const save = useSave(
    (v: { current_password: string; new_password: string }) => api.post("/auth/password/change", v),
    { success: "Kata sandi diganti. Sesi di perangkat lain dicabut.", onDone: () => setForm(EMPTY), onFieldErrors: setErrors },
  );

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const next: Record<string, string> = {};
    if (!form.current_password) next.current_password = "Kata sandi saat ini wajib diisi.";
    if (form.new_password.length < 6) next.new_password = "Kata sandi minimal 6 karakter.";
    if (form.new_password !== form.confirm) next.confirm = "Konfirmasi tidak sama.";
    setErrors(next);
    if (Object.keys(next).length === 0) {
      save.mutate({ current_password: form.current_password, new_password: form.new_password });
    }
  }

  return (
    <Card title="Ganti kata sandi">
      <form onSubmit={submit} noValidate className="space-y-4">
        <Field label="Kata sandi saat ini" htmlFor="current_password" error={errors.current_password}>
          <PasswordInput
            id="current_password"
            autoComplete="current-password"
            value={form.current_password}
            onChange={(e) => setForm({ ...form, current_password: e.target.value })}
            invalid={!!errors.current_password}
          />
        </Field>
        <Field label="Kata sandi baru" htmlFor="new_password" error={errors.new_password} hint="Minimal 6 karakter, hindari kata sandi umum.">
          <PasswordInput
            id="new_password"
            autoComplete="new-password"
            value={form.new_password}
            onChange={(e) => setForm({ ...form, new_password: e.target.value })}
            invalid={!!errors.new_password}
          />
        </Field>
        <Field label="Ulangi kata sandi baru" htmlFor="confirm" error={errors.confirm}>
          <PasswordInput
            id="confirm"
            autoComplete="new-password"
            value={form.confirm}
            onChange={(e) => setForm({ ...form, confirm: e.target.value })}
            invalid={!!errors.confirm}
          />
        </Field>
        <Button type="submit" loading={save.isPending}>
          Simpan kata sandi
        </Button>
      </form>
    </Card>
  );
}
