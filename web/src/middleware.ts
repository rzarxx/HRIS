import { NextResponse, type NextRequest } from "next/server";

// Cek ringan: halaman admin butuh cookie sesi. Validasi token sebenarnya tetap di API
// (respons 401 mengarahkan ke /login). Bila API eksternal (Go) di domain lain dipakai,
// cookie-nya tidak terlihat di sini, jadi pengecekan diserahkan ke API.
const EXTERNAL_API = Boolean(process.env.NEXT_PUBLIC_API_URL);

export function middleware(req: NextRequest) {
  if (EXTERNAL_API || req.cookies.has("hris_session")) return NextResponse.next();
  const url = req.nextUrl.clone();
  url.pathname = "/login";
  url.search = req.nextUrl.pathname === "/" ? "" : `?next=${encodeURIComponent(req.nextUrl.pathname)}`;
  return NextResponse.redirect(url);
}

export const config = {
  matcher: ["/((?!login|api|_next|favicon.ico|.*\\..*).*)"],
};
