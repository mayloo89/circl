"use client"

import { useState } from "react"
import { Link, usePathname } from "@/i18n/navigation"

const NAV_ITEMS = [
  { href: "/admin", label: "Dashboard" },
  { href: "/admin/users", label: "Users" },
  { href: "/admin/reports", label: "Reports" },
  { href: "/admin/appeals", label: "Appeals" },
  { href: "/admin/moderation", label: "Moderation" },
  { href: "/admin/channels", label: "Channels" },
  { href: "/admin/public-rooms", label: "Public rooms" },
]

export default function AdminShell({ children }: { children: React.ReactNode }) {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const pathname = usePathname()

  return (
    <div className="flex min-h-screen flex-col bg-gray-950">
      {/* Mobile backdrop */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/60 md:hidden"
          onClick={() => setSidebarOpen(false)}
          aria-hidden="true"
        />
      )}

      {/* Mobile slide-over sidebar */}
      <aside
        id="admin-sidebar"
        className={[
          "fixed inset-y-0 left-0 z-50 flex w-56 flex-col overflow-y-auto md:hidden",
          "border-r border-gray-800 bg-gray-900",
          "transition-transform duration-200 ease-out",
          sidebarOpen ? "translate-x-0" : "-translate-x-full",
        ].join(" ")}
      >
        <div className="flex items-start justify-between border-b border-gray-800 px-5 py-4">
          <div>
            <Link href="/" aria-label="Circl" className="flex items-center">
              {/* eslint-disable-next-line @next/next/no-img-element -- static SVG */}
              <img src="/branding/logo-dark.svg" alt="" width={100} height={28} className="h-7 w-auto" />
            </Link>
            <p className="mt-0.5 text-xs font-medium uppercase tracking-wider text-brand-muted">Admin</p>
          </div>
          <button
            onClick={() => setSidebarOpen(false)}
            className="mt-0.5 rounded p-1 text-gray-400 hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-hover"
            aria-label="Close navigation"
          >
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        <nav className="mt-2 flex flex-col gap-0.5 px-2 py-2">
          {NAV_ITEMS.map(({ href, label }) => {
            const active = href === "/admin" ? pathname === "/admin" : pathname.startsWith(href)
            return (
              <Link
                key={href}
                href={href}
                onClick={() => setSidebarOpen(false)}
                className={`block rounded px-3 py-2 text-sm transition-colors ${
                  active
                    ? "bg-gray-800 font-medium text-foreground"
                    : "text-gray-300 hover:bg-gray-800 hover:text-foreground"
                }`}
              >
                {label}
              </Link>
            )
          })}
        </nav>
      </aside>

      {/* Top header — logo + desktop tab nav + mobile hamburger */}
      <header className="flex items-stretch border-b border-gray-800 bg-gray-900 px-4">
        {/* Logo + Admin label */}
        <div className="flex shrink-0 items-center gap-3 py-3 pr-6">
          <Link href="/" aria-label="Circl home">
            {/* eslint-disable-next-line @next/next/no-img-element -- static SVG */}
            <img src="/branding/logo-dark.svg" alt="" width={80} height={24} className="h-6 w-auto" />
          </Link>
          <span className="text-xs font-semibold uppercase tracking-wider text-brand-muted">Admin</span>
        </div>

        {/* Desktop horizontal tab nav */}
        <nav className="hidden items-stretch gap-1 md:flex" aria-label="Admin navigation">
          {NAV_ITEMS.map(({ href, label }) => {
            const active = href === "/admin" ? pathname === "/admin" : pathname.startsWith(href)
            return (
              <Link
                key={href}
                href={href}
                className={[
                  "-mb-px flex items-center border-b-2 px-3 text-sm font-medium transition-colors",
                  active
                    ? "border-brand-primary text-foreground"
                    : "border-transparent text-gray-400 hover:border-gray-600 hover:text-gray-200",
                ].join(" ")}
              >
                {label}
              </Link>
            )
          })}
        </nav>

        {/* Mobile hamburger */}
        <button
          onClick={() => setSidebarOpen(true)}
          aria-label="Open navigation"
          aria-expanded={sidebarOpen}
          aria-controls="admin-sidebar"
          className="ml-auto flex items-center rounded p-1 text-gray-400 hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-hover md:hidden"
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
      </header>

      {/* Page content */}
      <main className="flex-1 overflow-auto">
        {children}
      </main>
    </div>
  )
}
