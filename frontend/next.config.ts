import type { NextConfig } from "next";
import type { RemotePattern } from "next/dist/shared/lib/image-config";

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

const nextConfig: NextConfig = {
  output: "standalone",
  reactCompiler: true,
  images: {
    remotePatterns,
    dangerouslyAllowLocalIP: process.env.NODE_ENV === "development",
  },
};

export default nextConfig;
