import type { NextConfig } from "next";

const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "no-referrer" },
  { key: "X-Frame-Options", value: "DENY" },
];

// Build-time version for the service worker URL (/sw.js?v=…): each build gets
// its own cache and a waiting-update flow.
const buildVersion = process.env.BUILD_VERSION ?? Date.now().toString(36);

const nextConfig: NextConfig = {
  env: { NEXT_PUBLIC_BUILD_VERSION: buildVersion },
  output: "standalone",
  poweredByHeader: false,
  // Agent instructions live outside src/ (see docs/development); stop `next dev`
  // from generating AGENTS.md/CLAUDE.md in this runtime project.
  agentRules: false,
  reactStrictMode: true,
  // This app is an independent project; never trace files above it.
  outputFileTracingRoot: import.meta.dirname,
  turbopack: { root: import.meta.dirname },
  async headers() {
    return [
      { source: "/:path*", headers: securityHeaders },
      // The worker script must always be revalidated so updates are found.
      { source: "/sw.js", headers: [{ key: "Cache-Control", value: "no-cache" }, { key: "Service-Worker-Allowed", value: "/" }] },
    ];
  },
};

export default nextConfig;
