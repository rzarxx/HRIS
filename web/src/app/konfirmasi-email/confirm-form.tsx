"use client";

import { CircleCheck, MailCheck } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { api, ApiError } from "@/lib/api-client";

type State = "loading" | "ready" | "done" | "invalid";

export function ConfirmEmailForm() {
  const [token, setToken] = useState("");
  const [state, setState] = useState<State>("loading");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const value = new URLSearchParams(window.location.hash.slice(1)).get("token") ?? "";
    // Hapus token dari bilah alamat dan riwayat begitu dibaca.
    if (window.location.hash) window.history.replaceState(null, "", window.location.pathname);
    setToken(value);
    setState(value ? "ready" : "invalid");
  }, []);

  // Konfirmasi lewat tombol, bukan otomatis saat halaman dibuka, agar pemindai tautan di aplikasi
  // email tidak ikut mengonfirmasi.
  async function confirm() {
    setBusy(true);
    setError("");
    try {
      await api.post("/auth/email/confirm", { token });
      setState("done");
    } catch (err) {
      if (err instanceof ApiError && err.code === "EMAIL_CHANGE_INVALID") setState("invalid");
      else setError(err instanceof ApiError ? err.message : "Belum terhubung ke server. Silakan periksa koneksi Anda.");
    } finally {
      setBusy(false);
    }
  }

  if (state === "loading") return null;

  if (state === "done") {
    return (
      <div className="space-y-4">
        <div role="status" className="flex items-start gap-2 rounded-md bg-primary-50 px-3 py-2 text-sm text-primary-700">
          <CircleCheck className="mt-0.5 size-4 shrink-0" aria-hidden />
          Email baru sudah berlaku. Semua sesi lama dikeluarkan; silakan masuk kembali dengan email baru.
        </div>
        <Link href="/login/" className="block text-center text-sm font-medium text-primary-700 hover:underline">
          Ke halaman masuk
        </Link>
      </div>
    );
  }

  if (state === "invalid") {
    return (
      <div className="space-y-4">
        <div role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          Tautan sudah tidak berlaku. Tautan hanya bisa dipakai sekali dan berlaku 24 jam. Silakan ajukan ganti email lagi dari
          Pengaturan di web atau Profil di aplikasi.
        </div>
        <Link href="/login/" className="block text-center text-sm font-medium text-primary-700 hover:underline">
          Ke halaman masuk
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {error && (
        <div role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {error}
        </div>
      )}
      <div className="flex items-start gap-2 rounded-md bg-slate-50 px-3 py-2 text-sm text-slate-600">
        <MailCheck className="mt-0.5 size-4 shrink-0" aria-hidden />
        Setelah dikonfirmasi, email lama tidak bisa dipakai masuk lagi dan semua sesi di web serta aplikasi dikeluarkan.
      </div>
      <Button type="button" className="w-full" loading={busy} onClick={confirm}>
        Konfirmasi email baru
      </Button>
    </div>
  );
}
