"use client";

import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo } from "react";
import { api } from "@/lib/api-client";
import { initials } from "@/lib/format";

const SIZES = { sm: "size-8 text-xs", md: "size-12 text-sm", lg: "size-20 text-xl" } as const;

/**
 * Foto profil Pejuang, atau inisial nama bila belum ada foto. Foto diambil lewat API dengan cookie sesi
 * (bukan <img src> langsung) karena endpoint foto butuh autentikasi; `version` (photo_updated_at)
 * membuat foto baru langsung tampil setelah diganti.
 */
export function Avatar({
  employeeId,
  name,
  hasPhoto,
  version,
  size = "sm",
}: {
  employeeId?: string;
  name?: string;
  hasPhoto?: boolean;
  version?: string | null;
  size?: keyof typeof SIZES;
}) {
  const photo = useQuery({
    queryKey: ["employee-photo", employeeId, version],
    queryFn: () => api.blob(`/employees/${employeeId}/photo`),
    enabled: !!employeeId && !!hasPhoto,
    staleTime: Infinity,
    retry: false,
  });
  const url = useMemo(() => (photo.data ? URL.createObjectURL(photo.data) : null), [photo.data]);
  useEffect(() => () => {
    if (url) URL.revokeObjectURL(url);
  }, [url]);

  const base = `flex shrink-0 items-center justify-center overflow-hidden rounded-full ${SIZES[size]}`;
  if (hasPhoto && url) {
    // eslint-disable-next-line @next/next/no-img-element -- blob URL dari API, bukan aset statis
    return <img src={url} alt={name ? `Foto ${name}` : "Foto profil"} className={`${base} object-cover`} />;
  }
  return (
    <span className={`${base} bg-secondary-400 font-semibold text-primary-900`} aria-hidden={!name}>
      {name ? initials(name) : "…"}
    </span>
  );
}
