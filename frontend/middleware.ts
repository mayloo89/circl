import createMiddleware from "next-intl/middleware"
import { NextResponse } from "next/server"

import { auth } from "@/lib/auth"
import { routing } from "./i18n/routing"

const intlMiddleware = createMiddleware(routing)

const AUTH_PAGES = [
  "/login",
  "/register",
  "/forgot-password",
  "/reset-password",
  "/verify-email",
]

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
  const isLoggedIn = !!req.auth
  const { pathname } = req.nextUrl
  const localePath = stripLocale(pathname)

  const isAuthPage = AUTH_PAGES.some((p) => localePath.startsWith(p))

  if (!isLoggedIn && !isAuthPage) {
    const locale = getLocale(pathname)
    return NextResponse.redirect(new URL(`/${locale}/login`, req.url))
  }

  if (isLoggedIn && isAuthPage) {
    const locale = getLocale(pathname)
    return NextResponse.redirect(new URL(`/${locale}`, req.url))
  }

  return intlMiddleware(req)
})

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico|.*\\..*).*)"],
}
