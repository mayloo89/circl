import { useTranslations } from "next-intl"
import { Link } from "@/i18n/navigation"

// Locale-scoped 404: rendered inside the locale layout for notFound() calls
// (e.g. unknown profile) and for unknown paths via the [...rest] catch-all,
// so translations are available.
export default function NotFound() {
  const t = useTranslations("notFound")

  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center px-6 text-center">
      <p className="text-6xl font-extrabold tracking-tight text-brand-primary" aria-hidden="true">
        404
      </p>
      <h1 className="mt-4 text-xl font-semibold text-gray-100">{t("title")}</h1>
      <p className="mt-3 max-w-md text-sm text-gray-400">{t("description")}</p>
      <Link
        href="/"
        className="mt-6 inline-flex items-center justify-center rounded-full bg-brand-primary px-6 py-3 text-sm font-bold text-white transition-colors hover:bg-brand-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-primary focus-visible:ring-offset-2 focus-visible:ring-offset-background"
      >
        {t("goHome")}
      </Link>
    </div>
  )
}
