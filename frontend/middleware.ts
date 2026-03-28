import { auth } from "@/lib/auth"
import { NextResponse } from "next/server"

const PUBLIC_PATHS = new Set(["/login", "/register"])

export default auth((req) => {
  const { pathname } = req.nextUrl

  if (!req.auth && !PUBLIC_PATHS.has(pathname)) {
    return NextResponse.redirect(new URL("/login", req.nextUrl))
  }

  if (req.auth && PUBLIC_PATHS.has(pathname)) {
    return NextResponse.redirect(new URL("/", req.nextUrl))
  }
})

export const config = {
  matcher: ["/((?!api/auth|_next/static|_next/image|favicon\\.ico).*)"],
}
