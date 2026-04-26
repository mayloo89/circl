"use client"

import type React from "react"
import { useTranslations } from "next-intl"
import { usePathname } from "@/i18n/navigation"
import { Link } from "@/i18n/navigation"
import { useNotificationsContext } from "@/contexts/NotificationsContext"
import Badge from "@/components/ui/Badge"

function HomeIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
      <polyline points="9 22 9 12 15 12 15 22" />
    </svg>
  )
}

function CompassIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="10" />
      <polygon points="16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76" />
    </svg>
  )
}

function ChatIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
    </svg>
  )
}

function UsersIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
      <circle cx="9" cy="7" r="4" />
      <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
      <path d="M16 3.13a4 4 0 0 1 0 7.75" />
    </svg>
  )
}

function PersonIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
      <circle cx="12" cy="7" r="4" />
    </svg>
  )
}

export default function BottomNav() {
  const t = useTranslations("nav")
  const pathname = usePathname()
  const { pendingCount, unreadChatCount } = useNotificationsContext()

  const items: { href: string; label: string; icon: React.ReactNode; badge?: number }[] = [
    { href: "/", label: t("home"), icon: <HomeIcon /> },
    { href: "/browse", label: t("browse"), icon: <CompassIcon /> },
    { href: "/chat", label: t("messages"), icon: <ChatIcon />, badge: unreadChatCount },
    { href: "/contacts", label: t("contacts"), icon: <UsersIcon />, badge: pendingCount },
    { href: "/profile", label: t("profile"), icon: <PersonIcon /> },
  ]

  return (
    <nav
      className="fixed bottom-0 left-0 right-0 z-40 flex h-16 items-stretch border-t border-gray-800 bg-gray-900 lg:hidden"
      style={{ paddingBottom: "env(safe-area-inset-bottom)" }}
      aria-label={t("mainNav")}
    >
      {items.map(({ href, label, icon, badge }) => {
        const isActive = href === "/" ? pathname === "/" : pathname.startsWith(href)
        return (
          <Link
            key={href}
            href={href}
            aria-current={isActive ? "page" : undefined}
            className={`relative flex flex-1 flex-col items-center justify-center gap-0.5 text-xs font-medium transition-colors ${
              isActive ? "text-brand-primary" : "text-gray-400 hover:text-gray-200"
            }`}
          >
            <span className="relative">
              {icon}
              {badge != null && badge > 0 && (
                <Badge count={badge} max={9} variant="dot" className="absolute -right-3 -top-1" />
              )}
            </span>
            <span>{label}</span>
          </Link>
        )
      })}
    </nav>
  )
}
