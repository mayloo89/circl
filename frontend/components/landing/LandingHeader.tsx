"use client"

import Image from "next/image"
import { useLocale, useTranslations } from "next-intl"

import { Link, usePathname, useRouter } from "@/i18n/navigation"
import { routing, type Locale } from "@/i18n/routing"
import { useTheme } from "@/contexts/ThemeContext"

const LOCALE_LABELS: Record<Locale, string> = { es: "ES", en: "EN", pt: "PT" }

export default function LandingHeader() {
  const t = useTranslations("landing")
  const router = useRouter()
  const pathname = usePathname()
  const currentLocale = useLocale()
  const { theme } = useTheme()

  return (
    <header className="sticky top-0 z-40 border-b border-gray-800/60 bg-background/80 backdrop-blur-md">
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-4 px-4 sm:px-6">
        <Link href="/" aria-label="Circl" className="shrink-0 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-hover">
          <Image
            src={theme === "dark" ? "/branding/logo-dark.svg" : "/branding/logo-light.svg"}
            alt="Circl"
            width={120}
            height={32}
            priority
            className="h-8 w-auto"
          />
        </Link>

        <div className="flex items-center gap-2 sm:gap-3">
          <div
            role="group"
            aria-label={t("language")}
            className="hidden items-center gap-0.5 rounded-full bg-gray-800/70 p-1 ring-1 ring-gray-700/70 sm:flex"
          >
            {routing.locales.map((locale) => {
              const active = locale === currentLocale
              return (
                <button
                  key={locale}
                  type="button"
                  aria-pressed={active}
                  onClick={() => router.replace(pathname, { locale })}
                  className={`min-w-[2.25rem] cursor-pointer rounded-full px-2.5 py-1 text-xs font-semibold transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-hover ${
                    active ? "bg-brand-primary text-white" : "text-gray-400 hover:text-foreground"
                  }`}
                >
                  {LOCALE_LABELS[locale]}
                </button>
              )
            })}
          </div>

          <Link
            href="/login"
            className="rounded-full px-3 py-2 text-sm font-semibold text-foreground/80 transition-colors hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-hover"
          >
            {t("navLogin")}
          </Link>
          <Link
            href="/register"
            className="rounded-full bg-brand-accent px-4 py-2 text-sm font-bold text-white shadow-sm transition-colors hover:bg-brand-accent/85 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-accent focus-visible:ring-offset-2 focus-visible:ring-offset-background"
          >
            {t("navJoin")}
          </Link>
        </div>
      </div>
    </header>
  )
}
