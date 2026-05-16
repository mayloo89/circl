"use client"

import { useEffect, useState } from "react"
import { useTranslations } from "next-intl"
import { use } from "react"

import Footer from "@/components/Footer"
import Button from "@/components/ui/Button"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface AppealView {
  status: "open" | "submitted" | "approved" | "denied" | "expired"
  body: string
  expires_at: string
  resolution_note?: string
}

export default function AppealPage({
  params,
}: {
  params: Promise<{ locale: string; token: string }>
}) {
  const { token } = use(params)
  const t = useTranslations("appeal")
  const [view, setView] = useState<AppealView | null>(null)
  const [body, setBody] = useState("")
  const [invalid, setInvalid] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const res = await fetch(`${API_URL}/appeal/${token}`)
        if (!res.ok) {
          if (!cancelled) setInvalid(true)
          return
        }
        const data: AppealView = await res.json()
        if (!cancelled) {
          setView(data)
          if (data.body) setBody(data.body)
        }
      } catch {
        if (!cancelled) setInvalid(true)
      }
    }
    load()
    return () => { cancelled = true }
  }, [token])

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/appeal/${token}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ body }),
      })
      if (!res.ok) {
        const text = await res.text()
        setError(text.trim() || "Failed to submit appeal")
        return
      }
      const data: AppealView = await res.json()
      setView(data)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-gray-950">
      <article className="mx-auto max-w-xl px-4 py-10">
        <header className="mb-6">
          <h1 className="font-display text-3xl font-bold text-white">{t("title")}</h1>
          <p className="mt-2 text-sm text-gray-400">{t("subtitle")}</p>
        </header>

        {invalid ? (
          <div className="rounded-md border border-rose-700 bg-rose-950/40 p-4 text-sm text-rose-200">
            <h2 className="font-medium mb-1">{t("invalidTokenHeading")}</h2>
            <p>{t("invalidTokenBody")}</p>
          </div>
        ) : view === null ? (
          <p className="text-sm text-gray-500">Loading…</p>
        ) : view.status === "approved" || view.status === "denied" || view.status === "expired" ? (
          <div className="rounded-md border border-gray-800 bg-gray-900 p-4 text-sm text-gray-200">
            <h2 className="font-medium mb-1">{t("alreadyResolvedHeading")}</h2>
            <p>
              {t("alreadyResolvedBody", {
                status:
                  view.status === "approved"
                    ? t("statusApproved")
                    : t("statusDenied"),
              })}
            </p>
            {view.resolution_note && (
              <p className="mt-3 whitespace-pre-wrap text-gray-300">{view.resolution_note}</p>
            )}
          </div>
        ) : view.status === "submitted" ? (
          <div className="rounded-md border border-green-800 bg-green-950/40 p-4 text-sm text-green-200">
            <p>{t("submitted")}</p>
            {view.body && (
              <p className="mt-3 whitespace-pre-wrap text-gray-200 text-sm">{view.body}</p>
            )}
          </div>
        ) : (
          <form onSubmit={submit} className="space-y-4">
            <div>
              <label htmlFor="body" className="block text-sm font-medium text-gray-200 mb-2">
                {t("bodyLabel")}
              </label>
              <textarea
                id="body"
                value={body}
                onChange={(e) => setBody(e.target.value)}
                rows={8}
                maxLength={4096}
                required
                className="w-full rounded-md border border-gray-700 bg-gray-900 px-3 py-2 text-sm text-gray-200 focus:outline-none focus:ring-1 focus:ring-brand-hover"
                placeholder={t("bodyPlaceholder")}
              />
            </div>

            <p className="text-xs text-gray-500">
              {t("expiresAt", { date: new Date(view.expires_at).toLocaleDateString() })}
            </p>

            {error && <p className="text-sm text-red-400">{error}</p>}

            <Button type="submit" variant="primary" loading={submitting}>
              {submitting ? t("submitting") : t("submit")}
            </Button>
          </form>
        )}
      </article>
      <Footer />
    </div>
  )
}
