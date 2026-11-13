// Client HTTP tunggal untuk semua halaman (MASTER.md §7, §10.1).
// Autentikasi lewat cookie HttpOnly; JavaScript tidak pernah memegang token.

import type { ErrorCode } from "./types";

const BASE_URL = (process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080").replace(/\/$/, "");

/** Pesan ramah per kode error; message dari server hanya cadangan (MASTER.md §7.7). */
const ERROR_MESSAGES: Partial<Record<ErrorCode, string>> = {
  AUTH_INVALID_CREDENTIALS: "Email atau kata sandi salah.",
  AUTH_ACCOUNT_LOCKED: "Akun terkunci. Coba lagi nanti atau hubungi SDM.",
  AUTH_MFA_INVALID: "Kode verifikasi salah.",
  ACTIVATION_INVALID: "Tautan aktivasi tidak valid. Periksa kembali tautannya atau minta tautan baru ke SDM.",
  ACTIVATION_USED: "Tautan ini sudah dipakai atau sudah diganti tautan baru. Silakan masuk dengan kata sandi Anda, atau minta tautan baru ke SDM.",
  ACTIVATION_EXPIRED: "Tautan ini sudah kedaluwarsa. Minta tautan baru ke SDM.",
  CONFLICT_HAS_DEPENDENTS: "Data masih dipakai data lain sehingga tidak bisa dihapus.",
  CONFLICT_SELF_ACTION: "Tindakan ini tidak dapat dilakukan pada akun atau perangkat Anda sendiri.",
  AUTH_TOKEN_INVALID: "Sesi berakhir, silakan masuk kembali.",
  FORBIDDEN_ROLE: "Anda tidak memiliki akses ke fitur ini.",
  CSRF_INVALID: "Token keamanan tidak valid. Muat ulang halaman lalu coba lagi.",
  NOT_FOUND: "Data tidak ditemukan.",
  RATE_LIMITED: "Terlalu banyak permintaan. Tunggu sebentar lalu coba lagi.",
  PAYLOAD_TOO_LARGE: "Ukuran data terlalu besar.",
  APPROVAL_SELF_FORBIDDEN: "Pengajuan Anda sendiri harus diputuskan approver lain.",
  APPROVAL_NOT_YOUR_TURN: "Belum giliran Anda. Langkah sebelumnya belum diputuskan.",
  REQUEST_NOT_PENDING: "Pengajuan ini sudah diputuskan atau dibatalkan. Muat ulang untuk melihat status terbaru.",
};

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public fields?: Record<string, string>,
    public requestId?: string,
    public details?: Record<string, unknown>,
  ) {
    super(message);
  }
}

type Query = Record<string, string | number | boolean | null | undefined>;

function buildUrl(path: string, query?: Query) {
  const url = `${BASE_URL}/v1${path}`;
  if (!query) return url;
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) {
    if (v !== undefined && v !== null && v !== "") params.set(k, String(v));
  }
  const qs = params.toString();
  return qs ? `${url}?${qs}` : url;
}

function csrfToken() {
  const match = document.cookie.match(/(?:^|;\s*)hris_csrf=([^;]+)/);
  return match ? decodeURIComponent(match[1]) : "";
}

function send(method: string, url: string, body?: unknown) {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (method !== "GET") headers["X-CSRF-Token"] = csrfToken();
  return fetch(url, {
    method,
    credentials: "include",
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

// Rotasi refresh token hanya boleh berjalan satu kali dalam satu waktu: token lama yang
// dipakai ulang akan dianggap pencurian dan sesi dicabut (MASTER.md F6).
let refreshing: Promise<boolean> | null = null;

function refreshSession() {
  refreshing ??= send("POST", buildUrl("/auth/web/refresh"))
    .then((res) => res.ok)
    .catch(() => false)
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

function redirectToLogin() {
  if (typeof window === "undefined" || window.location.pathname.startsWith("/login")) return;
  const next = window.location.pathname + window.location.search;
  window.location.href = `/login/?next=${encodeURIComponent(next)}`;
}

function redirectToSettings(code: string) {
  if (typeof window === "undefined") return;
  const params = new URLSearchParams(window.location.search);
  if (window.location.pathname.startsWith("/pengaturan") && params.get("wajib") === code) return;
  window.location.href = `/pengaturan/?wajib=${code}`;
}

async function request<T>(method: string, path: string, opts: { query?: Query; body?: unknown; blob?: boolean } = {}): Promise<T> {
  const url = buildUrl(path, opts.query);
  let res: Response;
  try {
    res = await send(method, url, opts.body);
    // Access token 15 menit kedaluwarsa: coba refresh sekali, lalu ulangi.
    if (res.status === 401 && !path.startsWith("/auth/")) {
      if (await refreshSession()) {
        res = await send(method, url, opts.body);
      }
    }
  } catch {
    throw new ApiError(0, "NETWORK_ERROR", "Tidak dapat terhubung ke server. Periksa koneksi Anda.");
  }

  if (res.status === 204) return undefined as T;

  if (res.ok && opts.blob) return (await res.blob()) as T;
  const payload = await res.json().catch(() => null);
  if (!res.ok) {
    const err = payload?.error;
    if (res.status === 401 && !path.startsWith("/auth/")) redirectToLogin();
    const code = (err?.code ?? "UNKNOWN") as ErrorCode;
    const requestId: string | undefined = err?.request_id;
    // CONFLICT_HAS_DEPENDENTS memakai pesan server karena lebih spesifik.
    let message: string = (code === "CONFLICT_HAS_DEPENDENTS" ? err?.message : ERROR_MESSAGES[code]) ?? err?.message ?? "Terjadi kesalahan, coba lagi.";
    if (code === "INTERNAL_ERROR" && requestId) message = `Terjadi kesalahan pada server. Kode: ${requestId}`;
    // Sandi sementara atau MFA wajib: arahkan ke halaman yang bisa menyelesaikannya.
    if (code === "PASSWORD_CHANGE_REQUIRED" || code === "MFA_SETUP_REQUIRED") redirectToSettings(code);
    throw new ApiError(res.status, code, message, err?.details?.fields, requestId, err?.details);
  }
  return payload as T;
}

export const api = {
  get: <T>(path: string, query?: Query) => request<T>("GET", path, { query }),
  blob: (path: string) => request<Blob>("GET", path, { blob: true }),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, { body }),
  patch: <T>(path: string, body?: unknown) => request<T>("PATCH", path, { body }),
  put: <T>(path: string, body?: unknown) => request<T>("PUT", path, { body }),
  del: <T = void>(path: string) => request<T>("DELETE", path),
};
