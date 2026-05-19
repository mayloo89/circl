"use client"

import type React from "react"
import { useTranslations } from "next-intl"
import { usePathname } from "@/i18n/navigation"
import { Link } from "@/i18n/navigation"
import { useNotificationsContext } from "@/contexts/NotificationsContext"
import Badge from "@/components/ui/Badge"

function HomeIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
      <polyline points="9 22 9 12 15 12 15 22" />
    </svg>
  )
}

function CompassIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="12" r="10" />
      <polygon points="16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76" />
    </svg>
  )
}

function ChatIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
    </svg>
  )
}

function UsersIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
      <circle cx="9" cy="7" r="4" />
      <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
      <path d="M16 3.13a4 4 0 0 1 0 7.75" />
    </svg>
  )
}


function AlbumIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="3" y="3" width="18" height="18" rx="2" />
      <circle cx="9" cy="9" r="2" />
      <path d="M21 15l-5-5L5 21" />
    </svg>
  )
}

function ChannelsIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
      <line x1="9" y1="10" x2="15" y2="10" />
      <line x1="9" y1="13" x2="13" y2="13" />
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
    { href: "/chat/channels", label: t("channels"), icon: <ChannelsIcon /> },
    { href: "/albums", label: t("albums"), icon: <AlbumIcon /> },
    { href: "/contacts", label: t("contacts"), icon: <UsersIcon />, badge: pendingCount },
  ]

  return (
    <nav
      className="fixed bottom-0 left-0 right-0 z-40 flex items-stretch border-t border-gray-800 bg-gray-900 lg:hidden"
      style={{
        height: "calc(4rem + env(safe-area-inset-bottom, 0px))",
        paddingBottom: "env(safe-area-inset-bottom, 0px)",
      }}
      aria-label={t("mainNav")}
    >
      {items.map(({ href, label, icon, badge }) => {
        const isActive = href === "/" ? pathname === "/" : href === "/chat" ? pathname === "/chat" || (pathname.startsWith("/chat/") && !pathname.startsWith("/chat/channels")) : pathname === href || pathname.startsWith(href + "/")
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
