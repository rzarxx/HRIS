"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { Avatar } from "@/components/avatar";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/form";
import { PasswordInput } from "@/components/ui/password-input";
import { Card } from "@/components/ui/layout";
import { useToast } from "@/components/ui/toast";
import { api } from "@/lib/api-client";
import { dateTime } from "@/lib/format";
import { useMe, useSave } from "@/lib/hooks";
import type { CurrentUser } from "@/lib/types";

const MAX_PHOTO_BYTES = 5 * 1024 * 1024;

type Contact = { phone: string; email: string; current_password: string };

/** Tampilan WA 62812… sebagai 0812… agar mudah dibaca. */
function localPhone(phone: string | null | undefined) {
  if (!phone) return "";
  return phone.startsWith("62") ? `0${phone.slice(2)}` : phone;
}

/**
 * Akun saya: foto profil, data dasar, serta nomor WA dan email yang bisa diubah sendiri. Perubahan
 * tersimpan di data Pejuang yang sama dengan aplikasi, jadi langsung tampil di kedua sisi.
 */
export function ProfileCard() {
  const { data: me } = useMe();
  const qc = useQueryClient();
  const fileRef = useRef<HTMLInputElement>(null);
  const [form, setForm] = useState<Contact>({ phone: "", email: "", current_password: "" });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const toast = useToast();

  useEffect(() => {
    if (me) setForm({ phone: localPhone(me.phone), email: me.email ?? "", current_password: "" });
  }, [me]);

  const applyMe = (res: { data: CurrentUser }) => qc.setQueryData<CurrentUser>(["me"], res.data);
  const saveContact = useSave((v: Partial<Contact>) => api.patch<{ data: CurrentUser }>("/me", v), {
    onDone: (res) => {
      // Bila SMTP aktif, email baru menunggu konfirmasi dari tautan di email itu (D59).
      const pending = res.data.pending_email;
      toast.success(pending && pending !== me?.pending_email ? `Tautan konfirmasi dikirim ke ${pending}. Email baru berlaku setelah tautan dibuka.` : "Kontak diperbarui.");
      applyMe(res);
      setErrors({});
    },
    onFieldErrors: setErrors,
  });
  const cancelEmail = useSave(() => api.del("/me/email-change"), {
    success: "Ganti email dibatalkan.",
    onDone: () => {
      if (me) qc.setQueryData<CurrentUser>(["me"], { ...me, pending_email: null });
    },
  });
  const savePhoto = useSave((file: File) => api.putFile<{ data: CurrentUser }>("/me/photo", file), {
    success: "Foto profil diperbarui.",
    onDone: applyMe,
    onFieldErrors: setErrors,
  });
  const removePhoto = useSave(() => api.del("/me/photo"), {
    success: "Foto profil dihapus.",
    onDone: () => {
      if (me) qc.setQueryData<CurrentUser>(["me"], { ...me, has_photo: false, photo_updated_at: null });
    },
  });

  const emailChanged = !!me && form.email.trim() !== (me.email ?? "");

  function pickPhoto(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    if (!["image/jpeg", "image/png"].includes(file.type)) {
      setErrors({ photo: "Mohon gunakan foto JPEG atau PNG." });
      return;
    }
    if (file.size > MAX_PHOTO_BYTES) {
      setErrors({ photo: "Ukuran foto paling besar 5 MB." });
      return;
    }
    setErrors({});
    savePhoto.mutate(file);
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!me) return;
    const body: Partial<Contact> = {};
    if (form.phone.trim() !== localPhone(me.phone)) body.phone = form.phone.trim() || undefined;
    if (form.phone.trim() === "" && me.phone) (body as Record<string, unknown>).phone = null;
    if (emailChanged) {
      if (!form.current_password) {
        setErrors({ current_password: "Masukkan kata sandi saat ini untuk mengganti email." });
        return;
      }
      (body as Record<string, unknown>).email = form.email.trim() || null;
      body.current_password = form.current_password;
    }
    if (Object.keys(body).length === 0) {
      setErrors({});
      return;
    }
    saveContact.mutate(body);
  }

  return (
    <Card title="Akun saya">
      <div className="flex items-center gap-4">
        <Avatar employeeId={me?.employee_id} name={me?.full_name} hasPhoto={me?.has_photo} version={me?.photo_updated_at} size="lg" />
        <div className="min-w-0 space-y-2">
          <p className="truncate font-semibold text-slate-900">{me?.full_name ?? "-"}</p>
          <div className="flex flex-wrap gap-2">
            <Button type="button" variant="secondary" size="sm" loading={savePhoto.isPending} onClick={() => fileRef.current?.click()}>
              {me?.has_photo ? "Ganti foto" : "Unggah foto"}
            </Button>
            {me?.has_photo && (
              <Button type="button" variant="ghost" size="sm" loading={removePhoto.isPending} onClick={() => removePhoto.mutate(undefined)}>
                Hapus foto
              </Button>
            )}
          </div>
          <input ref={fileRef} type="file" accept="image/jpeg,image/png" className="hidden" onChange={pickPhoto} aria-label="Pilih foto profil" />
          {errors.photo && <p className="text-xs text-red-600">{errors.photo}</p>}
        </div>
      </div>

      <dl className="mt-5 grid grid-cols-3 gap-y-2 text-sm">
        <dt className="text-slate-500">NIPY</dt>
        <dd className="col-span-2 font-mono text-xs text-slate-800">{me?.nip ?? "-"}</dd>
        <dt className="text-slate-500">Jabatan</dt>
        <dd className="col-span-2 text-slate-800">{me?.position ?? "-"}</dd>
        <dt className="text-slate-500">Unit</dt>
        <dd className="col-span-2 text-slate-800">{me ? `${me.subdivision.name} · ${me.division.name}` : "-"}</dd>
        <dt className="text-slate-500">Role</dt>
        <dd className="col-span-2 text-slate-800 uppercase">{me?.role ?? "-"}</dd>
      </dl>

      <form onSubmit={submit} noValidate className="mt-5 space-y-4 border-t border-slate-100 pt-4">
        <Field label="Nomor WA" htmlFor="phone" error={errors.phone} hint="Contoh 081234567890. Bisa dipakai untuk masuk.">
          <Input
            id="phone"
            inputMode="tel"
            autoComplete="tel"
            value={form.phone}
            onChange={(e) => setForm({ ...form, phone: e.target.value })}
            invalid={!!errors.phone}
          />
        </Field>
        <Field label="Email" htmlFor="email" error={errors.email}>
          <Input
            id="email"
            type="email"
            autoComplete="email"
            value={form.email}
            onChange={(e) => setForm({ ...form, email: e.target.value })}
            invalid={!!errors.email}
          />
        </Field>
        {me?.pending_email && (
          <div role="status" className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
            <span>
              Menunggu konfirmasi ke <span className="font-medium">{me.pending_email}</span>. Buka tautan di email itu dalam 24 jam.
            </span>
            <Button type="button" variant="ghost" size="sm" loading={cancelEmail.isPending} onClick={() => cancelEmail.mutate(undefined)}>
              Batalkan
            </Button>
          </div>
        )}
        {emailChanged && (
          <Field label="Kata sandi saat ini" htmlFor="profile_current_password" error={errors.current_password} hint="Diperlukan karena email juga dipakai untuk masuk.">
            <PasswordInput
              id="profile_current_password"
              autoComplete="current-password"
              value={form.current_password}
              onChange={(e) => setForm({ ...form, current_password: e.target.value })}
              invalid={!!errors.current_password}
            />
          </Field>
        )}
        <Button type="submit" loading={saveContact.isPending}>
          Simpan kontak
        </Button>
      </form>
      <p className="mt-4 text-xs text-slate-500">Waktu ditampilkan dalam WIB, contoh: {dateTime(new Date().toISOString())}.</p>
    </Card>
  );
}
