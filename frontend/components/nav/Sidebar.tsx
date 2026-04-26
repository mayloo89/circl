"use client"

import type React from "react"
import { signOut, useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { useLocale } from "next-intl"
import { usePathname, useRouter, Link } from "@/i18n/navigation"
import { routing, type Locale } from "@/i18n/routing"
import { useNotificationsContext } from "@/contexts/NotificationsContext"
import { useProfileContext } from "@/contexts/ProfileContext"
import { useSidebar } from "@/contexts/SidebarContext"
import Avatar from "@/components/ui/Avatar"
import Badge from "@/components/ui/Badge"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
const LOCALE_SHORT: Record<Locale, string> = { es: "ES", en: "EN", pt: "PT" }

function HomeIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
      <polyline points="9 22 9 12 15 12 15 22" />
    </svg>
  )
}

function CompassIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="10" />
      <polygon points="16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76" />
    </svg>
  )
}

function ChatIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
    </svg>
  )
}

function UsersIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
      <circle cx="9" cy="7" r="4" />
      <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
      <path d="M16 3.13a4 4 0 0 1 0 7.75" />
    </svg>
  )
}

function PersonIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
      <circle cx="12" cy="7" r="4" />
    </svg>
  )
}

function ChannelsIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
      <line x1="9" y1="10" x2="15" y2="10" />
      <line x1="9" y1="13" x2="13" y2="13" />
    </svg>
  )
}

function SettingsIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  )
}

function LogOutIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
      <polyline points="16 17 21 12 16 7" />
      <line x1="21" y1="12" x2="9" y2="12" />
    </svg>
  )
}

function CollapseIcon({ collapsed }: { collapsed: boolean }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className={`transition-transform duration-200 ${collapsed ? "rotate-180" : ""}`}>
      <polyline points="15 18 9 12 15 6" />
    </svg>
  )
}

export default function Sidebar() {
  const t = useTranslations("nav")
  const { data: session } = useSession()
  const { pendingCount, unreadChatCount } = useNotificationsContext()
  const { profile } = useProfileContext()
  const { collapsed, toggle } = useSidebar()
  const pathname = usePathname()
  const router = useRouter()
  const currentLocale = useLocale() as Locale

  const navItems: { href: string; label: string; icon: React.ReactNode; badge?: number }[] = [
    { href: "/", label: t("home"), icon: <HomeIcon /> },
    { href: "/browse", label: t("browse"), icon: <CompassIcon /> },
    { href: "/chat", label: t("messages"), icon: <ChatIcon />, badge: unreadChatCount },
    { href: "/chat/channels", label: t("channels"), icon: <ChannelsIcon /> },
    { href: "/contacts", label: t("contacts"), icon: <UsersIcon />, badge: pendingCount },
    { href: "/profile", label: t("profile"), icon: <PersonIcon /> },
  ]

  async function handleSignOut() {
    const token = session?.accessToken
    if (token) {
      try {
        await fetch(`${API_URL}/presence/heartbeat`, {
          method: "DELETE",
          headers: { Authorization: `Bearer ${token}` },
        })
      } catch {
        // non-critical
      }
    }
    if (session?.refreshToken) {
      try {
        await fetch(`${API_URL}/auth/logout`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refresh_token: session.refreshToken }),
        })
      } catch {
        // non-critical
      }
    }
    await signOut()
  }

  async function handleLocaleChange(locale: Locale) {
    if (session?.accessToken) {
      try {
        await fetch(`${API_URL}/profiles/me/preferences`, {
          method: "PUT",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${session.accessToken}`,
          },
          body: JSON.stringify({ locale }),
        })
      } catch {
        // non-critical
      }
    }
    router.replace(pathname, { locale })
  }

  const w = collapsed ? "w-16" : "w-64"
  const contentMargin = collapsed ? "lg:ml-16" : "lg:ml-64"

  return (
    <aside className={`fixed inset-y-0 left-0 z-40 hidden flex-col border-r border-gray-800 bg-gray-900 transition-all duration-200 lg:flex ${w}`}>
      {/* Logo + collapse toggle */}
      <div className={`flex h-16 items-center border-b border-gray-800 ${collapsed ? "justify-center px-0" : "justify-between px-4"}`}>
        {!collapsed && (
          <Link href="/" className="text-xl font-bold text-white hover:text-gray-200">
            Circl
          </Link>
        )}
        <button
          onClick={toggle}
          aria-label={collapsed ? t("expandSidebar") : t("collapseSidebar")}
          className={`rounded-md p-1.5 text-gray-400 hover:bg-gray-800 hover:text-white transition-colors ${collapsed ? "" : ""}`}
        >
          <CollapseIcon collapsed={collapsed} />
        </button>
      </div>

      {/* Nav items */}
      <nav className="flex-1 space-y-0.5 overflow-y-auto px-2 py-2" aria-label={t("mainNav")}>
        {navItems.map(({ href, label, icon, badge }) => {
          const isActive = href === "/" ? pathname === "/" : href === "/chat" ? pathname === "/chat" || pathname.startsWith("/chat/") && !pathname.startsWith("/chat/channels") : pathname === href || pathname.startsWith(href + "/")
          return (
            <Link
              key={href}
              href={href}
              aria-current={isActive ? "page" : undefined}
              title={collapsed ? label : undefined}
              className={`group flex items-center rounded-lg py-2.5 text-sm font-medium transition-colors ${collapsed ? "justify-center px-0" : "gap-3 px-3"} ${
                isActive
                  ? "bg-brand-primary/10 text-brand-primary"
                  : "text-gray-300 hover:bg-gray-800 hover:text-white"
              }`}
            >
              <span className="relative flex-shrink-0">
                {icon}
                {badge != null && badge > 0 && (
                  <Badge count={badge} max={9} variant="dot" className="absolute -right-2 -top-1" />
                )}
              </span>
              {!collapsed && label}
            </Link>
          )
        })}

        {(session?.role === "admin" || session?.role === "super_admin") && (
          <>
            <div className="my-2 border-t border-gray-800" />
            <Link
              href="/admin"
              aria-current={pathname.startsWith("/admin") ? "page" : undefined}
              title={collapsed ? t("adminPanel") : undefined}
              className={`flex items-center rounded-lg py-2.5 text-sm font-medium transition-colors ${collapsed ? "justify-center px-0" : "gap-3 px-3"} ${
                pathname.startsWith("/admin")
                  ? "bg-amber-500/10 text-amber-400"
                  : "text-amber-500 hover:bg-gray-800 hover:text-amber-400"
              }`}
            >
              {collapsed ? (
                <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
              ) : t("adminPanel")}
            </Link>
          </>
        )}
      </nav>

      {/* Bottom section */}
      <div className="border-t border-gray-800 px-2 py-3 space-y-0.5">
        <Link
          href="/settings"
          aria-current={pathname === "/settings" ? "page" : undefined}
          title={collapsed ? t("settings") : undefined}
          className={`flex items-center rounded-lg py-2.5 text-sm font-medium transition-colors ${collapsed ? "justify-center px-0" : "gap-3 px-3"} ${
            pathname === "/settings"
              ? "bg-brand-primary/10 text-brand-primary"
              : "text-gray-300 hover:bg-gray-800 hover:text-white"
          }`}
        >
          <SettingsIcon />
          {!collapsed && t("settings")}
        </Link>

        {/* Language switcher — only when expanded */}
        {!collapsed && (
          <div className="flex items-center gap-2 px-3 py-2">
            <span className="text-xs font-medium text-gray-500 flex-shrink-0">{t("language")}</span>
            <div className="flex gap-1">
              {routing.locales.map((locale) => (
                <button
                  key={locale}
                  aria-current={locale === currentLocale ? "true" : undefined}
                  onClick={() => handleLocaleChange(locale)}
                  className={`rounded px-2 py-1 text-xs font-medium transition-colors focus:outline-none focus:ring-1 focus:ring-brand-hover ${
                    locale === currentLocale
                      ? "bg-brand-primary text-white"
                      : "text-gray-400 hover:bg-gray-700 hover:text-white"
                  }`}
                >
                  {LOCALE_SHORT[locale]}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* User row + sign out */}
        <div className={`flex items-center rounded-lg py-2.5 ${collapsed ? "justify-center px-0" : "gap-3 px-3"}`}>
          <Link href="/profile" aria-label={t("profile")} title={collapsed ? (profile?.display_name ?? t("profile")) : undefined}>
            <Avatar
              src={profile?.avatar_url ?? ""}
              name={profile?.display_name || "?"}
              size="xs"
            />
          </Link>
          {!collapsed && (
            <>
              <span className="flex-1 truncate text-sm text-gray-300">
                {profile?.display_name}
              </span>
              <button
                onClick={handleSignOut}
                aria-label={t("logOut")}
                className="text-gray-500 hover:text-red-400 transition-colors"
              >
                <LogOutIcon />
              </button>
            </>
          )}
        </div>
      </div>

      {/* Invisible element to push content margin — consumed by AppShell via context */}
      <span data-sidebar-width={contentMargin} className="hidden" />
    </aside>
  )
}
