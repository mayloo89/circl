import { useTranslations } from "next-intl"
import { Link } from "@/i18n/navigation"

export default function Footer() {
  const t = useTranslations("footer")
  const year = new Date().getFullYear()

  const links = [
    { href: "/terms" as const, key: "terms" },
    { href: "/privacy" as const, key: "privacy" },
    { href: "/guidelines" as const, key: "guidelines" },
    { href: "/safety" as const, key: "safety" },
  ]

  return (
    <footer className="mt-8 w-full border-t border-gray-800 px-4 py-6 text-xs text-gray-500">
      <nav
        aria-label={t("navAriaLabel")}
        className="flex flex-col items-center gap-2 sm:flex-row sm:justify-between"
      >
        <ul className="flex flex-wrap items-center justify-center gap-x-4 gap-y-2">
          {links.map(({ href, key }) => (
            <li key={href}>
              <Link href={href} className="rounded hover:text-gray-300 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-brand-muted">
                {t(key)}
              </Link>
            </li>
          ))}
        </ul>
        <p>{t("copyright", { year })}</p>
      </nav>
    </footer>
  )
}
