import type { MetadataRoute } from "next"

// Authenticated / personal surfaces stay out of search engines; the public
// marketing landing, auth entry points, guest rooms, and legal pages remain
// crawlable. The default locale (es) is reachable unprefixed, so every private
// path is listed both bare and under each locale prefix.
const PRIVATE_PATHS = [
  "/chat",
  "/contacts",
  "/settings",
  "/admin",
  "/albums",
  "/onboarding",
  "/profile",
  "/browse",
  "/appeal",
  "/verify-email",
  "/reset-password",
  "/forgot-password",
]

const LOCALE_PREFIXES = ["", "/es", "/en", "/pt"]

export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: LOCALE_PREFIXES.flatMap((prefix) => PRIVATE_PATHS.map((path) => `${prefix}${path}`)),
    },
  }
}
