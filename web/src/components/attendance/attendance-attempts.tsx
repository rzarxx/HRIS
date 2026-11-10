"use client";

import { useState } from "react";
import { Input, Select } from "@/components/ui/form";
import { Badge, Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { Pagination, Table, Td, Th } from "@/components/ui/table";
import { todayWib, TZ } from "@/lib/format";
import { useAttendanceAttempts } from "@/lib/hooks";
import type { AttendanceAttempt } from "@/lib/types";

const PER_PAGE = 25;

const OUTCOME: Record<AttendanceAttempt["outcome"], { label: string; tone: "green" | "amber" | "red" }> = {
  accepted: { label: "Diterima", tone: "green" },
  review: { label: "Ditinjau", tone: "amber" },
  rejected: { label: "Ditolak", tone: "red" },
};

// Kode alasan MASTER.md §6.3. Hanya untuk peninjau (D8); kode yang belum dikenal ditampilkan apa adanya.
const REASON: Record<string, string> = {
  ok: "Lolos pemeriksaan",
  challenge_invalid: "Tantangan presensi tidak valid",
  challenge_expired: "Tantangan presensi kedaluwarsa",
  device_not_approved: "Perangkat belum disetujui atau bukan miliknya",
  integrity_failed: "Verifikasi perangkat gagal",
  integrity_nonce_mismatch: "Bukti integritas tidak cocok",
  hard_signal: "Fake GPS, root, atau emulator",
  geofence_out: "Di luar area kantor",
  shift_not_assigned: "Shift belum ditentukan",
  already_clocked_in: "Sudah presensi masuk",
  already_clocked_out: "Sudah presensi pulang",
  not_clocked_in: "Belum presensi masuk",
  face_not_enrolled: "Wajah belum terdaftar",
  face_not_detected: "Wajah tidak terdeteksi di selfie",
  liveness_failed: "Liveness gagal",
  face_mismatch: "Wajah tidak cocok",
  risk_too_high: "Skor risiko terlalu tinggi",
  risk_review: "Skor risiko perlu ditinjau",
  embedding_replay: "Selfie yang sama dikirim ulang",
};

const timeOf = (iso: string) => new Date(iso).toLocaleTimeString("id-ID", { timeZone: TZ, hour: "2-digit", minute: "2-digit", second: "2-digit" });

export function AttendanceAttempts() {
  const today = todayWib();
  const [date, setDate] = useState(today);
  const [outcome, setOutcome] = useState("");
  const [page, setPage] = useState(1);
  const list = useAttendanceAttempts({ date, outcome, page, per_page: PER_PAGE });

  return (
    <>
      <PageHeader title="Upaya Presensi" description="Semua upaya presensi, termasuk yang ditolak dan tidak pernah tercatat sebagai kehadiran." />
      <Card bodyClassName="">
        <div className="flex flex-wrap items-center gap-3 border-b border-slate-100 p-4">
          <Input
            type="date"
            value={date}
            max={today}
            onChange={(e) => {
              setDate(e.target.value || today);
              setPage(1);
            }}
            aria-label="Tanggal"
            className="w-full sm:w-auto"
          />
          <Select
            value={outcome}
            onChange={(e) => {
              setOutcome(e.target.value);
              setPage(1);
            }}
            aria-label="Filter hasil"
            className="w-full sm:w-auto sm:min-w-44"
          >
            <option value="">Semua hasil</option>
            <option value="rejected">{OUTCOME.rejected.label}</option>
            <option value="review">{OUTCOME.review.label}</option>
            <option value="accepted">{OUTCOME.accepted.label}</option>
          </Select>
        </div>
        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : list.data.data.length === 0 ? (
          <EmptyState title="Tidak ada upaya presensi" description="Ubah tanggal atau saringan hasil." />
        ) : (
          <>
            <Table>
              <thead>
                <tr>
                  <Th>Waktu (WIB)</Th>
                  <Th>Pejuang</Th>
                  <Th>Jenis</Th>
                  <Th>Hasil</Th>
                  <Th>Alasan</Th>
                  <Th align="right">Skor</Th>
                </tr>
              </thead>
              <tbody className={list.isPlaceholderData ? "opacity-60" : ""}>
                {list.data.data.map((a) => (
                  <tr key={a.id} className="hover:bg-slate-50">
                    <Td className="whitespace-nowrap tabular-nums">{timeOf(a.created_at)}</Td>
                    <Td>
                      <p className="font-medium text-slate-900">{a.employee_name}</p>
                      <p className="text-xs text-slate-500">
                        {a.nip} · {a.subdivision_name}
                      </p>
                    </Td>
                    <Td className="whitespace-nowrap">{a.purpose === "clock_in" ? "Masuk" : "Pulang"}</Td>
                    <Td>
                      <Badge tone={OUTCOME[a.outcome].tone}>{OUTCOME[a.outcome].label}</Badge>
                    </Td>
                    <Td className="min-w-48 text-sm text-slate-700">{REASON[a.reason_code] ?? a.reason_code}</Td>
                    <Td align="right" className="tabular-nums">
                      {a.risk_score}
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
            <Pagination page={page} perPage={PER_PAGE} total={list.data.meta.total} onPage={setPage} />
          </>
        )}
      </Card>
    </>
  );
}
