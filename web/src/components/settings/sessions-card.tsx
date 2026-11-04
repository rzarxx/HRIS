"use client";

import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Badge, Card, ErrorState, Loading } from "@/components/ui/layout";
import { api } from "@/lib/api-client";
import { dateTime } from "@/lib/format";
import { useSave } from "@/lib/hooks";
import type { ListResponse, Session } from "@/lib/types";

export function SessionsCard() {
  const sessions = useQuery({
    queryKey: ["sessions"],
    queryFn: () => api.get<ListResponse<Session>>("/auth/sessions").then((r) => r.data),
  });
  const revoke = useSave((id: string) => api.del(`/auth/sessions/${id}`), { success: "Sesi dicabut." });

  return (
    <Card title="Sesi aktif" bodyClassName="divide-y divide-slate-100">
      {sessions.isPending ? (
        <Loading />
      ) : sessions.isError ? (
        <ErrorState message={sessions.error.message} onRetry={() => sessions.refetch()} />
      ) : (
        sessions.data.map((s) => (
          <div key={s.id} className="flex items-center justify-between gap-3 px-5 py-3">
            <div className="min-w-0">
              <p className="truncate text-sm text-slate-800">{s.user_agent ?? "Perangkat tidak dikenal"}</p>
              <p className="text-xs text-slate-500">
                {s.client === "web" ? "Web" : "Aplikasi HP"} · {s.ip ?? "-"} · terakhir {dateTime(s.last_used_at)}
              </p>
            </div>
            {s.is_current ? (
              <Badge tone="accent">Sesi ini</Badge>
            ) : (
              <Button variant="secondary" size="sm" loading={revoke.isPending && revoke.variables === s.id} onClick={() => revoke.mutate(s.id)}>
                Cabut
              </Button>
            )}
          </div>
        ))
      )}
    </Card>
  );
}
