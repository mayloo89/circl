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

// CSP and HSTS are production-only: in dev, cross-origin API calls to
// localhost:8080 and Next.js hydration scripts would break under a strict policy.
const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  ...(isProd
    ? [
        {
          key: "Content-Security-Policy",
          value: [
            "default-src 'self'",
            // Next.js App Router requires 'unsafe-inline' for bootstrap hydration scripts.
            "script-src 'self' 'unsafe-inline' https://cdn.growthbook.io",
            "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
            "img-src 'self' data: blob: https:",
            "font-src 'self' https://fonts.gstatic.com",
            "connect-src 'self' wss: https://cdn.growthbook.io https://photon.komoot.io",
            "frame-ancestors 'none'",
          ].join("; "),
        },
        { key: "Strict-Transport-Security", value: "max-age=31536000; includeSubDomains" },
      ]
    : []),
];

const nextConfig: NextConfig = {
  output: "standalone",
  reactCompiler: true,
  devIndicators: false,
  images: {
    remotePatterns,
    dangerouslyAllowLocalIP: process.env.NODE_ENV === "development",
    unoptimized: process.env.NEXT_PUBLIC_IMAGE_UNOPTIMIZED === "true",
  },
  async headers() {
    return [{ source: "/(.*)", headers: securityHeaders }];
  },
};

export default withNextIntl(nextConfig);
