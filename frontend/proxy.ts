import createMiddleware from "next-intl/middleware"
import { NextRequest, NextResponse } from "next/server"

import { auth } from "@/lib/auth"
import { routing } from "./i18n/routing"

function buildCSP(nonce: string): string {
  return [
    "default-src 'self'",
    // No 'unsafe-inline': Next.js bootstrap scripts are stamped with the nonce.
    // challenges.cloudflare.com is the Turnstile anti-bot widget (script + iframe).
    `script-src 'self' 'nonce-${nonce}' https://cdn.growthbook.io https://challenges.cloudflare.com`,
    "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
    "img-src 'self' data: blob: https:",
    "font-src 'self' https://fonts.gstatic.com",
    "connect-src 'self' wss: https://cdn.growthbook.io https://photon.komoot.io https://challenges.cloudflare.com",
    "frame-src https://challenges.cloudflare.com",
    "frame-ancestors 'none'",
    "object-src 'none'",
  ].join("; ")
}

const intlMiddleware = createMiddleware(routing)

const AUTH_PAGES = [
  "/login",
  "/register",
  "/forgot-password",
  "/reset-password",
  "/verify-email",
]

// Legal/policy pages must be reachable to anyone — including unauthenticated
// visitors arriving from the registration consent links — and must not
// redirect logged-in users back to home, so they're treated as a separate
// public-readable bucket from AUTH_PAGES.
const PUBLIC_PAGES = ["/terms", "/privacy", "/guidelines", "/safety", "/appeal", "/rooms"]

// Marketing/acquisition surface search engines may index. Everything else gets
// an X-Robots-Tag: noindex header — robots.txt (app/robots.ts) only stops
// crawling, while this header keeps privately-linked URLs (profiles, chats,
// appeal tokens) out of the index even when discovered.
const INDEXABLE_PAGES = ["/", "/login", "/register", "/terms", "/privacy", "/guidelines", "/safety", "/rooms"]

function getLocale(pathname: string): string {
  return (
    routing.locales.find(
      (l) => pathname.startsWith(`/${l}/`) || pathname === `/${l}`,
    ) ?? routing.defaultLocale
  )
}

function stripLocale(pathname: string): string {
  for (const locale of routing.locales) {
    if (pathname.startsWith(`/${locale}/`)) return pathname.slice(locale.length + 1)
    if (pathname === `/${locale}`) return "/"
  }
  return pathname
}

export default auth((req) => {
  const isProd = process.env.NODE_ENV === "production"
  const nonce = isProd ? btoa(crypto.randomUUID()) : null
  const csp = nonce ? buildCSP(nonce) : null

  // Apply CSP and harden the NEXT_LOCALE locale cookie (Secure + HttpOnly).
  // next-intl sets the cookie via low-level Set-Cookie headers, not the Next.js
  // cookies API, so res.cookies.get("NEXT_LOCALE") is unreliable. Instead we
  // derive the locale from the incoming pathname — falling back to the default
  // locale via getLocale() for unprefixed requests (localePrefix is "as-needed",
  // so the default locale has no path segment) — and always write a hardened
  // cookie so the flags are guaranteed on every request, prefixed or not.
  const finalize = (res: NextResponse): NextResponse => {
    if (csp) res.headers.set("Content-Security-Policy", csp)
    const localePath = stripLocale(req.nextUrl.pathname)
    const indexable = INDEXABLE_PAGES.some((p) =>
      p === "/" ? localePath === "/" : localePath === p || localePath.startsWith(`${p}/`),
    )
    if (!indexable) res.headers.set("X-Robots-Tag", "noindex")
    res.cookies.set("NEXT_LOCALE", getLocale(req.nextUrl.pathname), {
      httpOnly: true,
      secure: isProd,
      sameSite: "lax",
      path: "/",
      maxAge: 60 * 60 * 24 * 365,
    })
    return res
  }

  // NextAuth sets req.auth to a truthy *error object* ({ message: "There was
  // a problem with the server configuration…" }) instead of null when session
  // resolution fails (e.g. UntrustedHost). Checking req.auth.user instead of
  // req.auth keeps the auth wall failing closed on misconfiguration — an
  // anonymous visitor must never be treated as logged in.
  const isLoggedIn = !!req.auth?.user
  const { pathname } = req.nextUrl
  const localePath = stripLocale(pathname)
  const isAuthPage = AUTH_PAGES.some((p) => localePath.startsWith(p))
  const isPublicPage = PUBLIC_PAGES.some((p) => localePath.startsWith(p))
  // The home route serves the public marketing landing to logged-out visitors
  // and the authenticated home to a session; the page itself branches on auth.
  const isHome = localePath === "/"

  if (!isLoggedIn && !isAuthPage && !isPublicPage && !isHome) {
    const locale = getLocale(pathname)
    return finalize(NextResponse.redirect(new URL(`/${locale}/login`, req.url)))
  }

  if (isLoggedIn && isAuthPage) {
    const locale = getLocale(pathname)
    return finalize(NextResponse.redirect(new URL(`/${locale}`, req.url)))
  }

  if (nonce && csp) {
    // Inject the nonce into the request headers so that:
    // - Next.js automatically stamps its own bootstrap scripts with it.
    // - Server Components can read it via headers().get('x-nonce').
    // Setting CSP in request headers lets Next.js extract the nonce value.
    const requestHeaders = new Headers(req.headers)
    requestHeaders.set("x-nonce", nonce)
    requestHeaders.set("Content-Security-Policy", csp)

    const modifiedReq = new NextRequest(req.url, {
      headers: requestHeaders,
      method: req.method,
    })
    return finalize(intlMiddleware(modifiedReq) as NextResponse)
  }

  return finalize(intlMiddleware(req) as NextResponse)
})

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico|.*\\..*).*)"],
}
