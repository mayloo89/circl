import type { NextConfig } from "next";
import type { RemotePattern } from "next/dist/shared/lib/image-config";
import createNextIntlPlugin from "next-intl/plugin";

const withNextIntl = createNextIntlPlugin("./i18n/request.ts");

// Allow images from localhost in dev. In production, the image host is
// configured via NEXT_PUBLIC_IMAGE_HOSTNAME (set as a Docker build arg).
const remotePatterns: RemotePattern[] = [
  { protocol: "http", hostname: "localhost", port: "8080" },
  { protocol: "http", hostname: "localhost", port: "9000" },
];

if (process.env.NEXT_PUBLIC_IMAGE_HOSTNAME) {
  remotePatterns.push({
    protocol:
      (process.env.NEXT_PUBLIC_IMAGE_PROTOCOL as "http" | "https") ?? "http",
    hostname: process.env.NEXT_PUBLIC_IMAGE_HOSTNAME,
    port: process.env.NEXT_PUBLIC_IMAGE_PORT ?? "",
  });
}

const isProd = process.env.NODE_ENV === "production";

const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  {
    key: "Permissions-Policy",
    value: "camera=(), microphone=(), geolocation=()",
  },
  {
    key: "Content-Security-Policy",
    value: [
      "default-src 'self'",
      // Next.js requires 'unsafe-inline' for its runtime styles and
      // 'unsafe-eval' is needed only in development for hot-reload.
      isProd ? "script-src 'self'" : "script-src 'self' 'unsafe-eval'",
      "style-src 'self' 'unsafe-inline'",
      "img-src 'self' data: blob:",
      "font-src 'self'",
      "connect-src 'self' ws: wss:",
      "frame-ancestors 'none'",
    ].join("; "),
  },
  ...(isProd
    ? [{ key: "Strict-Transport-Security", value: "max-age=31536000; includeSubDomains" }]
    : []),
];

const nextConfig: NextConfig = {
  output: "standalone",
  reactCompiler: true,
  images: {
    remotePatterns,
    dangerouslyAllowLocalIP: process.env.NODE_ENV === "development",
  },
  async headers() {
    return [{ source: "/(.*)", headers: securityHeaders }];
  },
};

export default withNextIntl(nextConfig);
