"use client"

import { useTranslations } from "next-intl"
import { usePathname, Link } from "@/i18n/navigation"
import { usePushContext } from "@/contexts/PushContext"
import { useProfileContext } from "@/contexts/ProfileContext"
import Avatar from "@/components/ui/Avatar"

function BellIcon({ muted }: { muted?: boolean }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {muted ? (
        <>
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
          <path d="M18.63 13A17.9 17.9 0 0 1 18 8" />
          <path d="M6.26 6.26A5.86 5.86 0 0 0 6 8c0 7-3 9-3 9h14" />
          <path d="M18 8a6 6 0 0 0-9.33-5" />
          <line x1="1" y1="1" x2="23" y2="23" />
        </>
      ) : (
        <>
          <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
        </>
      )}
    </svg>
  )
}

const ROUTE_TITLES: Record<string, string> = {
  "/": "home",
  "/browse": "browse",
  "/chat": "messages",
  "/contacts": "contacts",
  "/profile": "profile",
  "/settings": "settings",
  "/admin": "adminPanel",
}

export default function TopBar() {
  const t = useTranslations("nav")
  const pathname = usePathname()
  const { permission, supported, enable, disable } = usePushContext()
  const { profile } = useProfileContext()

  const titleKey = Object.entries(ROUTE_TITLES)
    .reverse()
    .find(([route]) => pathname === route || (route !== "/" && pathname.startsWith(route)))?.[1] ?? "home"

  return (
    <header className="fixed left-0 right-0 top-0 z-40 flex h-14 items-center border-b border-gray-800 bg-gray-900 px-4 lg:hidden">
      <Link href="/" className="text-base font-bold text-white">
        Circl
      </Link>

      <span className="ml-3 text-sm font-medium text-gray-400">
        {t(titleKey as Parameters<typeof t>[0])}
      </span>

      <div className="ml-auto flex items-center gap-3">
        {supported && permission !== "granted" && (
          <button
            onClick={enable}
            aria-label={t("enablePush")}
            className="p-1.5 text-gray-400 hover:text-white transition-colors"
          >
            <BellIcon muted />
          </button>
        )}
        {supported && permission === "granted" && (
          <button
            onClick={disable}
            aria-label={t("disablePush")}
            className="p-1.5 text-green-400 hover:text-gray-400 transition-colors"
          >
            <BellIcon />
          </button>
        )}

        <Link href="/profile" aria-label={t("profile")}>
          <Avatar
            src={profile?.avatar_url ?? ""}
            name={profile?.display_name || "?"}
            size="xs"
          />
        </Link>
      </div>
    </header>
  )
}
