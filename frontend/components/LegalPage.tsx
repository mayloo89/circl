import { useTranslations } from "next-intl"
import { type ReactNode } from "react"

import { Link } from "@/i18n/navigation"
import Footer from "@/components/Footer"

export interface LegalPageProps {
  /** Page heading rendered inside the article header. */
  title: string
  /** ISO date when the policy was last meaningfully updated (e.g. "2026-05-10"). */
  lastUpdated: string
  /** When true, renders an amber banner indicating the text is a draft awaiting legal review. */
  draft?: boolean
  children: ReactNode
}

export default function LegalPage({ title, lastUpdated, draft, children }: LegalPageProps) {
  const t = useTranslations("legal")

  return (
    <div className="min-h-screen bg-gray-950">
      <article className="mx-auto max-w-2xl px-4 py-10">
        <header className="mb-6">
          <Link
            href="/"
            className="mb-4 inline-flex items-center gap-1 text-sm text-gray-500 hover:text-gray-300 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-brand-muted rounded"
          >
            <span aria-hidden="true">←</span>
            {t("back")}
          </Link>
          <h1 className="font-display text-3xl font-bold text-foreground">{title}</h1>
          <p className="mt-2 text-sm text-gray-500">
            {t("lastUpdated", { date: lastUpdated })}
          </p>
        </header>

        {draft && (
          <div
            className="mb-6 rounded-md border border-amber-700 bg-amber-950/40 p-4 text-sm text-amber-200"
            role="note"
          >
            {t("draftBanner")}
          </div>
        )}

        <div className="legal-prose">{children}</div>
      </article>

      <Footer />
    </div>
  )
}
