"use client";

import { useQueryClient } from "@tanstack/react-query";
import { KeyRound } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/form";
import { PasswordInput } from "@/components/ui/password-input";
import { Modal } from "@/components/ui/modal";
import { api } from "@/lib/api-client";
import { useSave } from "@/lib/hooks";

const EMPTY = { current_password: "", new_password: "", confirm: "" };

/**
 * Popup wajib ganti kata sandi setelah masuk dengan kata sandi awal (D53). Tidak bisa ditutup;
 * satu-satunya jalan lain adalah keluar. Server tetap menolak fitur lain dengan PASSWORD_CHANGE_REQUIRED.
 */
export function ForcedPasswordModal({ name }: { name: string }) {
  const [form, setForm] = useState(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const qc = useQueryClient();
  const save = useSave(
    (v: { current_password: string; new_password: string }) => api.post("/auth/password/change", v),
    {
      success: "Kata sandi diganti. Selamat bekerja.",
      // useSave tidak memuat ulang profil; must_change_password harus segar agar popup tertutup.
      onDone: () => qc.invalidateQueries({ queryKey: ["me"] }),
      onFieldErrors: setErrors,
    },
  );

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const next: Record<string, string> = {};
    if (!form.current_password) next.current_password = "Kata sandi awal wajib diisi.";
    if (form.new_password.length < 10) next.new_password = "Kata sandi minimal 10 karakter.";
    else if (form.new_password === form.current_password) next.new_password = "Kata sandi baru harus berbeda dari kata sandi awal.";
    if (form.new_password !== form.confirm) next.confirm = "Konfirmasi tidak sama.";
    setErrors(next);
    if (Object.keys(next).length === 0) {
      save.mutate({ current_password: form.current_password, new_password: form.new_password });
    }
  }

  async function logout() {
    await api.post("/auth/logout").catch(() => {});
    window.location.href = "/login/";
  }

  return (
    <Modal
      open
      dismissible={false}
      size="sm"
      title="Ganti kata sandi dulu"
      onClose={() => {}}
      footer={
        <>
          <Button variant="secondary" onClick={logout}>
            Keluar
          </Button>
          <Button type="submit" form="forced-password-form" loading={save.isPending}>
            Simpan kata sandi
          </Button>
        </>
      }
    >
      <div className="mb-4 flex items-start gap-2 rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-800">
        <KeyRound className="mt-0.5 size-4 shrink-0" aria-hidden />
        <p>
          Assalamu&apos;alaikum, {name}. Anda masuk dengan kata sandi awal dari SDM. Demi keamanan akun, buat kata sandi sendiri sekarang sebelum memakai fitur lain.
        </p>
      </div>
      <form id="forced-password-form" onSubmit={submit} noValidate className="space-y-4">
        <Field label="Kata sandi awal" htmlFor="forced-current" error={errors.current_password}>
          <PasswordInput
            id="forced-current"
            autoComplete="current-password"
            autoFocus
            value={form.current_password}
            onChange={(e) => setForm({ ...form, current_password: e.target.value })}
            invalid={!!errors.current_password}
          />
        </Field>
        <Field label="Kata sandi baru" htmlFor="forced-new" error={errors.new_password} hint="Minimal 10 karakter, hindari kata sandi umum.">
          <PasswordInput
            id="forced-new"
            autoComplete="new-password"
            value={form.new_password}
            onChange={(e) => setForm({ ...form, new_password: e.target.value })}
            invalid={!!errors.new_password}
          />
        </Field>
        <Field label="Ulangi kata sandi baru" htmlFor="forced-confirm" error={errors.confirm}>
          <PasswordInput
            id="forced-confirm"
            autoComplete="new-password"
            value={form.confirm}
            onChange={(e) => setForm({ ...form, confirm: e.target.value })}
            invalid={!!errors.confirm}
          />
        </Field>
      </form>
    </Modal>
  );
}
