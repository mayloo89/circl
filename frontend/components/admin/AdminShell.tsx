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
]

function SidebarNav({ onNavigate }: { onNavigate?: () => void }) {
  const pathname = usePathname()
  return (
    <nav className="mt-2 px-2 py-2 flex flex-col gap-0.5">
      {NAV_ITEMS.map(({ href, label }) => {
        const active = href === "/admin"
          ? pathname === "/admin"
          : pathname.startsWith(href)
        return (
          <Link
            key={href}
            href={href}
            onClick={onNavigate}
            className={`block rounded px-3 py-2 text-sm transition-colors ${
              active
                ? "bg-gray-800 text-foreground font-medium"
                : "text-gray-300 hover:bg-gray-800 hover:text-foreground"
            }`}
          >
            {label}
          </Link>
        )
      })}
    </nav>
  )
}

export default function AdminShell({ children }: { children: React.ReactNode }) {
  const [sidebarOpen, setSidebarOpen] = useState(false)

  return (
    <div className="flex min-h-screen bg-gray-950">
      {/* Mobile backdrop */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/60 md:hidden"
          onClick={() => setSidebarOpen(false)}
          aria-hidden="true"
        />
      )}

      {/* Sidebar — fixed slide-over on mobile, static on desktop */}
      <aside
        id="admin-sidebar"
        className={[
          "fixed inset-y-0 left-0 z-50 flex w-56 shrink-0 flex-col overflow-y-auto",
          "border-r border-gray-800 bg-gray-900",
          "transition-transform duration-200 ease-out",
          sidebarOpen ? "translate-x-0" : "-translate-x-full",
          "md:relative md:translate-x-0 md:transition-none",
        ].join(" ")}
      >
        <div className="flex items-start justify-between px-5 py-4 border-b border-gray-800">
          <div>
            <Link href="/" aria-label="Circl" className="flex items-center">
              {/* eslint-disable-next-line @next/next/no-img-element -- static SVG, next/image adds unnecessary overhead */}
              <img
                src="/branding/logo-dark.svg"
                alt=""
                width={100}
                height={28}
                className="h-7 w-auto"
              />
            </Link>
            <p className="mt-0.5 text-xs font-medium text-brand-muted uppercase tracking-wider">Admin</p>
          </div>
          <button
            onClick={() => setSidebarOpen(false)}
            className="md:hidden mt-0.5 rounded p-1 text-gray-400 hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-hover"
            aria-label="Close navigation"
          >
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <SidebarNav onNavigate={() => setSidebarOpen(false)} />
      </aside>

      {/* Content column */}
      <div className="flex flex-1 flex-col min-w-0">
        {/* Mobile top bar */}
        <header className="flex items-center gap-3 border-b border-gray-800 bg-gray-900 px-4 py-3 md:hidden">
          <button
            onClick={() => setSidebarOpen(true)}
            aria-label="Open navigation"
            aria-expanded={sidebarOpen}
            aria-controls="admin-sidebar"
            className="rounded p-1 text-gray-400 hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-hover"
          >
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>
          <span className="text-sm font-medium text-brand-muted uppercase tracking-wider">Admin</span>
        </header>

        <main className="flex-1 overflow-auto">
          {children}
        </main>
      </div>
    </div>
  )
}
