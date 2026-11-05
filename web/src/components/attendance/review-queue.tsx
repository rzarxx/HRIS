"use client";

import dynamic from "next/dynamic";
import { useMemo, useState } from "react";
import type { MapPoint } from "@/components/attendance/attendance-points-map";
import { Button } from "@/components/ui/button";
import { Field, Select, Textarea } from "@/components/ui/form";
import { Badge, Card, EmptyState, ErrorState, Loading, PageHeader } from "@/components/ui/layout";
import { Modal } from "@/components/ui/modal";
import { Pagination, Table, Td, Th } from "@/components/ui/table";
import { api } from "@/lib/api-client";
import { dateLong, dateTime } from "@/lib/format";
import { useAttendanceReview, useMe, useSave } from "@/lib/hooks";
import type { AttendanceResolveInput, AttendanceReviewItem } from "@/lib/types";

const AttendancePointsMap = dynamic(() => import("@/components/attendance/attendance-points-map"), {
  ssr: false,
  loading: () => <Loading label="Memuat peta..." />,
});

const PER_PAGE = 20;

const WORK_MODE: Record<AttendanceReviewItem["work_mode"], string> = {
  onsite: "Kantor",
  wfa: "WFA",
  field: "Lapangan",
};

// Alasan ini hanya tampil untuk peninjau (D8). Penanda yang belum dikenal ditampilkan apa adanya.
const FLAG_LABEL: Record<string, string> = {
  accuracy_low: "Akurasi GPS rendah",
  location_jump: "Berpindah lokasi terlalu cepat",
  clock_skew: "Jam HP berbeda dari server",
  new_device_recent: "Perangkat baru disetujui",
  off_schedule: "Di luar jadwal shift",
  unverified_integrity: "Tanpa verifikasi perangkat (development)",
};

function pointsOf(a: AttendanceReviewItem): MapPoint[] {
  const points: MapPoint[] = [];
  if (a.in_latitude !== null && a.in_longitude !== null) points.push({ label: "Masuk", latitude: a.in_latitude, longitude: a.in_longitude });
  if (a.out_latitude !== null && a.out_longitude !== null) points.push({ label: "Pulang", latitude: a.out_latitude, longitude: a.out_longitude });
  return points;
}

function ReviewModal({ item, onClose }: { item: AttendanceReviewItem; onClose: () => void }) {
  const me = useMe();
  // Keputusan memengaruhi upah, jadi server menolak bila peninjau memutuskan absennya sendiri.
  const isSelf = me.data?.employee_id === item.employee_id;
  const isLate = item.late_minutes > 0;
  const [decision, setDecision] = useState<AttendanceResolveInput["decision"]>(isLate ? "late" : "present");
  const [note, setNote] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const points = useMemo(() => pointsOf(item), [item]);

  const save = useSave((v: AttendanceResolveInput) => api.post(`/admin/attendance/${item.id}/resolve`, v), {
    success: "Keputusan tersimpan.",
    onDone: onClose,
    onFieldErrors: setErrors,
  });

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const text = note.trim();
    if (decision === "absent" && text.length < 3) {
      setErrors({ note: "Catatan wajib diisi bila absen tidak valid, minimal 3 karakter." });
      return;
    }
    setErrors({});
    save.mutate({ decision, note: text });
  }

  return (
    <Modal
      open
      size="lg"
      title={`Tinjau absen ${item.employee_name}`}
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Batal
          </Button>
          {!isSelf && (
            <Button type="submit" form="review-form" loading={save.isPending}>
              Simpan keputusan
            </Button>
          )}
        </>
      }
    >
      <dl className="grid grid-cols-3 gap-y-2 text-sm">
        <dt className="text-slate-500">Pejuang</dt>
        <dd className="col-span-2 break-words text-slate-800">
          {item.employee_name} · {item.nip}
        </dd>
        <dt className="text-slate-500">Unit</dt>
        <dd className="col-span-2 text-slate-800">
          {item.subdivision_name} · {item.division_name}
        </dd>
        <dt className="text-slate-500">Tanggal kerja</dt>
        <dd className="col-span-2 text-slate-800">{dateLong(item.work_date)}</dd>
        <dt className="text-slate-500">Masuk</dt>
        <dd className="col-span-2 text-slate-800">
          {dateTime(item.clock_in_at)}
          {isLate && <span className="ml-1 text-amber-800">(telat {item.late_minutes} menit)</span>}
        </dd>
        <dt className="text-slate-500">Pulang</dt>
        <dd className="col-span-2 text-slate-800">{item.clock_out_at ? dateTime(item.clock_out_at) : "Belum absen pulang"}</dd>
        <dt className="text-slate-500">Mode</dt>
        <dd className="col-span-2 text-slate-800">
          {WORK_MODE[item.work_mode]}
          {item.location_name && ` · ${item.location_name}`}
          {item.flexible_hours && " · Jam fleksibel"}
        </dd>
        <dt className="text-slate-500">Akurasi GPS</dt>
        <dd className="col-span-2 text-slate-800">{item.in_accuracy_m === null ? "-" : `${item.in_accuracy_m} m`}</dd>
      </dl>

      <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 px-3 py-2">
        <p className="text-sm font-medium text-amber-900">Skor risiko {item.risk_score}</p>
        <ul className="mt-1 list-disc pl-5 text-sm text-amber-900">
          {item.risk_flags.map((f) => (
            <li key={f}>{FLAG_LABEL[f] ?? f}</li>
          ))}
        </ul>
      </div>

      {points.length > 0 && (
        <div className="mt-4">
          <AttendancePointsMap points={points} />
        </div>
      )}

      {isSelf ? (
        <p className="mt-4 text-sm text-slate-500">Absen Anda sendiri harus ditinjau peninjau lain.</p>
      ) : (
        <form id="review-form" onSubmit={submit} noValidate className="mt-4 grid grid-cols-1 gap-4 border-t border-slate-100 pt-4">
          <Field label="Keputusan" htmlFor="review_decision" error={errors.decision} required>
            <Select id="review_decision" value={decision} onChange={(e) => setDecision(e.target.value as AttendanceResolveInput["decision"])} invalid={!!errors.decision}>
              {isLate && <option value="late">Valid, tetap telat {item.late_minutes} menit</option>}
              <option value="present">{isLate ? "Valid, hapus keterlambatan" : "Valid, hadir"}</option>
              <option value="absent">Tidak valid, tidak hadir</option>
            </Select>
          </Field>
          <Field label="Catatan" htmlFor="review_note" error={errors.note} required={decision === "absent"} hint={decision === "absent" ? undefined : "Opsional."}>
            <Textarea id="review_note" rows={2} maxLength={500} value={note} onChange={(e) => setNote(e.target.value)} invalid={!!errors.note} />
          </Field>
        </form>
      )}
    </Modal>
  );
}

export function ReviewQueue() {
  const [workMode, setWorkMode] = useState("");
  const [page, setPage] = useState(1);
  const list = useAttendanceReview({ work_mode: workMode, page, per_page: PER_PAGE });
  const [reviewing, setReviewing] = useState<AttendanceReviewItem | null>(null);

  return (
    <>
      <PageHeader title="Review Absen" description="Absen berisiko menunggu keputusan. Belum dihitung hadir sampai diputuskan." />
      <Card bodyClassName="">
        <div className="border-b border-slate-100 p-4">
          <Select
            value={workMode}
            onChange={(e) => {
              setWorkMode(e.target.value);
              setPage(1);
            }}
            aria-label="Filter mode kerja"
            className="w-full sm:w-auto sm:min-w-48"
          >
            <option value="">Semua mode</option>
            <option value="onsite">{WORK_MODE.onsite}</option>
            <option value="wfa">{WORK_MODE.wfa}</option>
            <option value="field">{WORK_MODE.field}</option>
          </Select>
        </div>
        {list.isPending ? (
          <Loading />
        ) : list.isError ? (
          <ErrorState message={list.error.message} onRetry={() => list.refetch()} />
        ) : list.data.data.length === 0 ? (
          <EmptyState title="Tidak ada absen yang perlu ditinjau" description="Absen berisiko akan muncul di sini." />
        ) : (
          <>
            <Table actions="always">
              <thead>
                <tr>
                  <Th>Pejuang</Th>
                  <Th>Tanggal kerja</Th>
                  <Th>Mode</Th>
                  <Th align="right">Skor</Th>
                  <Th>Sinyal</Th>
                  <Th align="right">
                    <span className="sr-only">Aksi</span>
                  </Th>
                </tr>
              </thead>
              <tbody className={list.isPlaceholderData ? "opacity-60" : ""}>
                {list.data.data.map((a) => (
                  <tr key={a.id} className="hover:bg-slate-50">
                    <Td>
                      <p className="font-medium text-slate-900">{a.employee_name}</p>
                      <p className="text-xs text-slate-500">
                        {a.nip} · {a.subdivision_name}
                      </p>
                    </Td>
                    <Td className="whitespace-nowrap">{dateLong(a.work_date)}</Td>
                    <Td>{WORK_MODE[a.work_mode]}</Td>
                    <Td align="right">
                      <Badge tone="amber">{a.risk_score}</Badge>
                    </Td>
                    <Td className="min-w-48 text-xs text-slate-600">{a.risk_flags.map((f) => FLAG_LABEL[f] ?? f).join(", ")}</Td>
                    <Td align="right">
                      <Button variant="secondary" size="sm" onClick={() => setReviewing(a)} aria-label={`Tinjau absen ${a.employee_name}, ${dateLong(a.work_date)}`}>
                        Tinjau
                      </Button>
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
            <Pagination page={page} perPage={PER_PAGE} total={list.data.meta.total} onPage={setPage} />
          </>
        )}
      </Card>

      {reviewing && <ReviewModal item={reviewing} onClose={() => setReviewing(null)} />}
    </>
  );
}
