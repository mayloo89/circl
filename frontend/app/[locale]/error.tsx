"use client"

import { useEffect } from "react"
import { useTranslations } from "next-intl"
import Button from "@/components/ui/Button"
import { reportClientError } from "@/lib/reportError"

// Locale-scoped error boundary: catches render/runtime errors in the app tree,
// reports them to the backend ingest endpoint, and offers a retry. Rendered
// inside the locale layout, so translations are available.
export default function LocaleError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  const t = useTranslations("error")

  useEffect(() => {
    reportClientError({ error, kind: "boundary" })
  }, [error])

  return (
    <div
      role="alert"
      className="flex min-h-[60vh] flex-col items-center justify-center px-6 text-center"
    >
      <h1 className="text-xl font-semibold text-gray-100">{t("title")}</h1>
      <p className="mt-3 max-w-md text-sm text-gray-400">{t("description")}</p>
      <Button variant="primary" size="lg" pill className="mt-6" onClick={reset}>
        {t("retry")}
      </Button>
    </div>
  )
}
