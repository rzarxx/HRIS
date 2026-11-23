"use client";

import { MailCheck, MailX } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input, Select } from "@/components/ui/form";
import { Badge, Card, ErrorState, Loading } from "@/components/ui/layout";
import { ConfirmDialog } from "@/components/ui/modal";
import { PasswordInput } from "@/components/ui/password-input";
import { api } from "@/lib/api-client";
import { dateTime } from "@/lib/format";
import { useMe, useSave, useSmtpSettings } from "@/lib/hooks";
import type { SmtpInput, SmtpSettings } from "@/lib/types";

type Form = { host: string; port: string; username: string; password: string; from_name: string; from_email: string };

const EMPTY: Form = { host: "", port: "587", username: "", password: "", from_name: "HRIS Al Bahjah", from_email: "" };

function fromSettings(s: SmtpSettings): Form {
  if (!s.configured) return EMPTY;
  return { host: s.host, port: String(s.port), username: s.username, password: "", from_name: s.from_name, from_email: s.from_email };
}

/**
 * Kartu pengaturan email (SMTP) untuk lupa kata sandi dan pemberitahuan keamanan; hanya super admin
 * (settings:manage). Pengaturan baru disimpan server setelah email uji ke akun ini terkirim.
 */
export function SmtpCard() {
  const smtp = useSmtpSettings();
  const { data: me } = useMe();
  const [form, setForm] = useState<Form>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [confirmOff, setConfirmOff] = useState(false);

  useEffect(() => {
    if (smtp.data) setForm(fromSettings(smtp.data));
  }, [smtp.data]);

  const save = useSave((body: SmtpInput) => api.put<{ data: SmtpSettings }>("/admin/settings/smtp", body), {
    success: "Email uji terkirim dan pengaturan email disimpan.",
    onDone: () => setForm((f) => ({ ...f, password: "" })),
    onFieldErrors: setErrors,
  });
  const disable = useSave(() => api.del("/admin/settings/smtp"), {
    success: "Pengiriman email dimatikan.",
    onDone: () => setConfirmOff(false),
  });

  const set = (key: keyof Form) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    setForm({ ...form, [key]: e.target.value });
    setErrors((prev) => ({ ...prev, [key]: "" }));
  };

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const next: Record<string, string> = {};
    if (!form.host.trim()) next.host = "Host SMTP wajib diisi.";
    if (!form.from_email.trim()) next.from_email = "Email pengirim wajib diisi.";
    if (form.username.trim() && !form.password && !smtp.data?.password_set) next.password = "Kata sandi SMTP wajib diisi.";
    setErrors(next);
    if (Object.keys(next).length > 0) return;
    save.mutate({
      host: form.host.trim(),
      port: form.port === "465" ? 465 : 587,
      username: form.username.trim(),
      password: form.password,
      from_name: form.from_name.trim(),
      from_email: form.from_email.trim(),
    });
  }

  const data = smtp.data;

  return (
    <Card
      title="Email (SMTP)"
      actions={data && (data.configured ? <Badge tone="green">Aktif</Badge> : <Badge>Belum diatur</Badge>)}
    >
      <p className="mb-4 text-sm text-slate-500">
        Dipakai untuk tautan lupa kata sandi dan pemberitahuan keamanan akun. Saat disimpan, email uji dikirim ke{" "}
        <span className="font-medium text-slate-700">{me?.email ?? "email akun Anda"}</span>; pengaturan baru hanya tersimpan bila email uji
        terkirim.
      </p>
      {smtp.isPending ? (
        <Loading />
      ) : smtp.isError ? (
        <ErrorState message={smtp.error.message} onRetry={() => smtp.refetch()} />
      ) : (
        <form onSubmit={submit} noValidate className="flex flex-col gap-4">
          {!me?.email && (
            <div role="alert" className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
              Isi email akun Anda lebih dulu di kartu Akun saya untuk menerima email uji.
            </div>
          )}
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-[1fr_7rem]">
            <Field label="Host SMTP" htmlFor="smtp_host" error={errors.host}>
              <Input
                id="smtp_host"
                value={form.host}
                onChange={set("host")}
                placeholder="smtp.gmail.com"
                autoCapitalize="none"
                spellCheck={false}
                invalid={!!errors.host}
              />
            </Field>
            <Field label="Port" htmlFor="smtp_port" error={errors.port} hint="587 STARTTLS, 465 SSL/TLS">
              <Select id="smtp_port" value={form.port} onChange={set("port")} invalid={!!errors.port}>
                <option value="587">587</option>
                <option value="465">465</option>
              </Select>
            </Field>
          </div>
          <Field label="Username" htmlFor="smtp_username" error={errors.username} hint="Biasanya alamat email akun SMTP.">
            <Input
              id="smtp_username"
              value={form.username}
              onChange={set("username")}
              autoComplete="off"
              autoCapitalize="none"
              spellCheck={false}
              invalid={!!errors.username}
            />
          </Field>
          <Field
            label="Kata sandi SMTP"
            htmlFor="smtp_password"
            error={errors.password}
            hint={data?.password_set ? "Kosongkan untuk tetap memakai kata sandi yang tersimpan." : "Untuk Gmail, pakai sandi aplikasi (App Password)."}
          >
            <PasswordInput
              id="smtp_password"
              value={form.password}
              onChange={set("password")}
              autoComplete="new-password"
              placeholder={data?.password_set ? "Tersimpan" : ""}
              invalid={!!errors.password}
            />
          </Field>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Nama pengirim" htmlFor="smtp_from_name" error={errors.from_name}>
              <Input id="smtp_from_name" value={form.from_name} onChange={set("from_name")} maxLength={80} invalid={!!errors.from_name} />
            </Field>
            <Field label="Email pengirim" htmlFor="smtp_from_email" error={errors.from_email}>
              <Input
                id="smtp_from_email"
                type="email"
                value={form.from_email}
                onChange={set("from_email")}
                placeholder="hris@albahjah.or.id"
                autoCapitalize="none"
                spellCheck={false}
                invalid={!!errors.from_email}
              />
            </Field>
          </div>
          {errors.test_to && <p className="text-sm text-red-600">{errors.test_to}</p>}
          <div className="flex flex-wrap items-center gap-2">
            <Button type="submit" loading={save.isPending} disabled={!me?.email}>
              <MailCheck className="size-4" aria-hidden /> Simpan dan kirim uji
            </Button>
            {data?.configured && (
              <Button type="button" variant="secondary" onClick={() => setConfirmOff(true)} disabled={save.isPending}>
                <MailX className="size-4" aria-hidden /> Matikan email
              </Button>
            )}
            {data?.configured && data.updated_at && <span className="ml-auto text-xs text-slate-500">Diubah {dateTime(data.updated_at)}</span>}
          </div>
        </form>
      )}
      <ConfirmDialog
        open={confirmOff}
        title="Matikan pengiriman email?"
        message="Lupa kata sandi lewat email dan pemberitahuan keamanan berhenti dikirim. Pejuang yang lupa kata sandi perlu menghubungi SDM."
        confirmLabel="Matikan"
        loading={disable.isPending}
        onConfirm={() => disable.mutate(undefined)}
        onClose={() => setConfirmOff(false)}
      />
    </Card>
  );
}
