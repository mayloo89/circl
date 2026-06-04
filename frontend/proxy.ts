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

  const addCSP = (res: NextResponse): NextResponse => {
    if (csp) res.headers.set("Content-Security-Policy", csp)
    return res
  }

  const isLoggedIn = !!req.auth
  const { pathname } = req.nextUrl
  const localePath = stripLocale(pathname)
  const isAuthPage = AUTH_PAGES.some((p) => localePath.startsWith(p))
  const isPublicPage = PUBLIC_PAGES.some((p) => localePath.startsWith(p))

  if (!isLoggedIn && !isAuthPage && !isPublicPage) {
    const locale = getLocale(pathname)
    return addCSP(NextResponse.redirect(new URL(`/${locale}/login`, req.url)))
  }

  if (isLoggedIn && isAuthPage) {
    const locale = getLocale(pathname)
    return addCSP(NextResponse.redirect(new URL(`/${locale}`, req.url)))
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
    return addCSP(intlMiddleware(modifiedReq) as NextResponse)
  }

  return intlMiddleware(req)
})

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico|.*\\..*).*)"],
}
