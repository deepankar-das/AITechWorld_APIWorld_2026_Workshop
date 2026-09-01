import type { NextConfig } from "next";
import path from "node:path";
import { fileURLToPath } from "node:url";

const consoleRoot = path.dirname(fileURLToPath(import.meta.url));

const nextConfig: NextConfig = {
  output: "export",
  // Static export — no Node.js server needed at runtime.
  // The built HTML/JS/CSS is embedded in the Go binary via go:embed.
  trailingSlash: true,
  images: {
    unoptimized: true,
  },
  turbopack: {
    root: consoleRoot,
  },
};

export default nextConfig;
