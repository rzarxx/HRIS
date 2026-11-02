"use client";

import { useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, ShieldCheck } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/form";
import { Badge, Card } from "@/components/ui/layout";
import { useToast } from "@/components/ui/toast";
import { api, ApiError } from "@/lib/api-client";
import { useMe } from "@/lib/hooks";
import type { ItemResponse, MfaEnabled, MfaSetup } from "@/lib/types";

export function MfaCard() {
  const { data: me } = useMe();
  const qc = useQueryClient();
  const toast = useToast();
  const [setup, setSetup] = useState<MfaSetup | null>(null);
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [recovery, setRecovery] = useState<string[] | null>(null);
  const [loading, setLoading] = useState(false);

  async function start() {
    setLoading(true);
    setError("");
    try {
      const res = await api.post<ItemResponse<MfaSetup>>("/auth/mfa/setup");
      setSetup(res.data);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Gagal memulai pendaftaran.");
    } finally {
      setLoading(false);
    }
  }

  async function enable(e: React.FormEvent) {
    e.preventDefault();
    if (!code.trim()) {
      setError("Kode wajib diisi.");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const res = await api.post<ItemResponse<MfaEnabled>>("/auth/mfa/enable", { code: code.trim() });
      setRecovery(res.data.recovery_codes);
      setSetup(null);
      await qc.invalidateQueries({ queryKey: ["me"] });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal mengaktifkan.");
    } finally {
      setLoading(false);
    }
  }

  if (recovery) {
    return (
      <Card title="Verifikasi dua langkah">
        <div className="flex items-start gap-2 text-sm text-emerald-700">
          <CheckCircle2 className="mt-0.5 size-4 shrink-0" aria-hidden />
          Verifikasi dua langkah aktif. Simpan kode pemulihan ini di tempat aman: masing-masing hanya bisa dipakai sekali dan
          tidak akan ditampilkan lagi.
        </div>
        <ul className="mt-3 grid grid-cols-2 gap-2 rounded-md bg-slate-50 p-3 font-mono text-sm text-slate-800">
          {recovery.map((c) => (
            <li key={c}>{c}</li>
          ))}
        </ul>
        <Button variant="secondary" className="mt-4" onClick={() => setRecovery(null)}>
          Sudah saya simpan
        </Button>
      </Card>
    );
  }

  return (
    <Card title="Verifikasi dua langkah" actions={me?.mfa_enabled ? <Badge tone="green">Aktif</Badge> : <Badge tone="amber">Belum aktif</Badge>}>
      {me?.mfa_enabled ? (
        <div className="flex items-start gap-2 text-sm text-slate-600">
          <ShieldCheck className="mt-0.5 size-4 shrink-0 text-emerald-600" aria-hidden />
          Saat masuk, Anda diminta kode dari aplikasi authenticator.
        </div>
      ) : setup ? (
        <form onSubmit={enable} noValidate className="space-y-4">
          <p className="text-sm text-slate-600">Pindai kode QR dengan aplikasi authenticator (Google Authenticator, Authy, dll.), lalu masukkan 6 digit kodenya.</p>
          <div className="flex justify-center rounded-md border border-slate-200 bg-white p-3">
            <QRCodeSVG value={setup.otpauth_url} size={176} />
          </div>
          <p className="text-xs break-all text-slate-500">
            Atau masukkan manual: <span className="font-mono text-slate-700">{setup.secret}</span>
          </p>
          <Field label="Kode verifikasi" htmlFor="mfa_code" error={error}>
            <Input
              id="mfa_code"
              inputMode="numeric"
              autoComplete="one-time-code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              invalid={!!error}
              className="font-mono tracking-widest"
            />
          </Field>
          <Button type="submit" loading={loading}>
            Aktifkan
          </Button>
        </form>
      ) : (
        <>
          <p className="text-sm text-slate-600">Tambahkan lapisan keamanan dengan kode dari aplikasi authenticator di HP Anda. Wajib untuk role SDM.</p>
          <Button className="mt-4" loading={loading} onClick={start}>
            Mulai pendaftaran
          </Button>
        </>
      )}
    </Card>
  );
}
