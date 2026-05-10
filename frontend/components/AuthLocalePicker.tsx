"use client"

import { useLocale } from "next-intl"
import { usePathname, useRouter } from "@/i18n/navigation"
import { routing, type Locale } from "@/i18n/routing"

// Fixed-position locale switcher rendered in the corner of unauthenticated
// pages (login, register, forgot-password, reset-password, verify-email) so
// users can correct an unwanted locale guess from the URL prefix or
// `Accept-Language` header before signing in. Routing through next-intl's
// `router.replace` updates the URL prefix and the NEXT_LOCALE cookie so the
// preference persists into the session.
const LABELS: Record<Locale, string> = {
  es: "ES",
  en: "EN",
  pt: "PT",
}

export default function AuthLocalePicker() {
  const router = useRouter()
  const pathname = usePathname()
  const currentLocale = useLocale()

  return (
    <div
      role="group"
      aria-label="Language"
      className="fixed right-4 top-4 z-30 flex gap-1 rounded-full bg-gray-900/80 p-1 shadow-lg ring-1 ring-gray-700 backdrop-blur-sm"
    >
      {routing.locales.map((locale) => {
        const active = locale === currentLocale
        return (
          <button
            key={locale}
            type="button"
            aria-pressed={active}
            onClick={() => router.replace(pathname, { locale })}
            className={`min-w-[2.5rem] rounded-full px-3 py-1 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-brand-hover ${
              active
                ? "bg-brand-primary text-white"
                : "text-gray-400 hover:text-white"
            }`}
          >
            {LABELS[locale]}
          </button>
        )
      })}
    </div>
  )
}
