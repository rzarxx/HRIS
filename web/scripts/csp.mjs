// Memasang Content-Security-Policy (MASTER.md §5.8) ke setiap halaman hasil `next build`.
// Web berupa file statis, jadi tidak ada nonce per permintaan. Skrip inline buatan Next.js diizinkan
// lewat hash sha256 isinya, sehingga script-src tetap tanpa 'unsafe-inline'. CSP dipasang sebagai
// <meta> di awal <head> agar berlaku sebelum skrip mana pun dimuat; frame-ancestors tidak berlaku di
// <meta>, dan perlindungan bingkai tetap dari header X-Frame-Options di Nginx.
import { createHash } from "node:crypto";
import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import nextEnv from "@next/env";

// Baca .env.local / .env.production seperti `next build`, agar alamat API sama dengan yang dibundel.
nextEnv.loadEnvConfig(new URL("..", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"));

const OUT = new URL("../out/", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1");

function apiOrigin() {
  const raw = process.env.NEXT_PUBLIC_API_URL?.trim();
  if (!raw) return null;
  try {
    return new URL(raw).origin;
  } catch {
    throw new Error(`NEXT_PUBLIC_API_URL tidak valid: ${raw}`);
  }
}

function htmlFiles(dir) {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return htmlFiles(path);
    return name.endsWith(".html") ? [path] : [];
  });
}

const connect = ["'self'", apiOrigin(), "https://nominatim.openstreetmap.org"].filter(Boolean);
const INLINE_SCRIPT = /<script(?![^>]*\bsrc=)[^>]*>([\s\S]*?)<\/script>/g;

let pages = 0;
for (const file of htmlFiles(OUT)) {
  let html = readFileSync(file, "utf8");
  if (html.includes('http-equiv="Content-Security-Policy"')) continue;
  const hashes = new Set();
  for (const [, body] of html.matchAll(INLINE_SCRIPT)) {
    if (body.length > 0) hashes.add(`'sha256-${createHash("sha256").update(body, "utf8").digest("base64")}'`);
  }
  const policy = [
    "default-src 'self'",
    `script-src 'self' ${[...hashes].join(" ")}`.trim(),
    // Tailwind dan Leaflet memakai atribut style; style tidak bisa menjalankan kode.
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob: https://tile.openstreetmap.org",
    "font-src 'self'",
    `connect-src ${[...new Set(connect)].join(" ")}`,
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
  ].join("; ");
  const head = html.indexOf("<head>");
  if (head < 0) throw new Error(`Tag <head> tidak ditemukan di ${file}`);
  const meta = `<meta http-equiv="Content-Security-Policy" content="${policy}"/>`;
  html = html.slice(0, head + 6) + meta + html.slice(head + 6);
  writeFileSync(file, html);
  pages++;
}
console.log(`CSP dipasang di ${pages} halaman (connect-src: ${connect.join(" ")}).`);
