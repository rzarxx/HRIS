// Serve the worker and font data from this site, never from a public CDN.
import { cpSync, mkdirSync } from "node:fs";
const source = new URL("../node_modules/pdfjs-dist/", import.meta.url);
const destination = new URL("../public/pdfjs/", import.meta.url);
mkdirSync(destination, { recursive: true });
cpSync(new URL("build/pdf.worker.min.mjs", source), new URL("pdf.worker.min.mjs", destination));
for (const directory of ["cmaps", "standard_fonts", "wasm", "iccs"]) {
  cpSync(new URL(directory, source), new URL(directory, destination), { recursive: true });
}
