import type { NextConfig } from "next";

// Web admin berupa file statis; semua data dari API Go (MASTER.md §4, §10.1).
const nextConfig: NextConfig = {
  output: "export",
  trailingSlash: true,
};

export default nextConfig;
