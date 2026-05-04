"use client"

import { useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { usePushContext } from "@/contexts/PushContext"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export default function PushPrompt() {
  const t = useTranslations("pushPrompt")
  const { data: session, status } = useSession()
  const { permission, supported, enable } = usePushContext()
  const [dismissed, setDismissed] = useState(false)
  const [eligible, setEligible] = useState(false)

  useEffect(() => {
    if (status !== "authenticated" || !session?.accessToken) return
    const fallback = setTimeout(() => setEligible(true), 30_000)
    fetch(`${API_URL}/chat/rooms`, {
      headers: { Authorization: `Bearer ${session.accessToken}` },
    })
      .then((r) => (r.ok ? r.json() : Promise.reject()))
      .then((data: unknown) => {
        if (Array.isArray(data) && data.length > 0) {
          clearTimeout(fallback)
          setEligible(true)
        }
      })
      .catch(() => {})
    return () => clearTimeout(fallback)
  }, [status, session?.accessToken])

  if (!eligible || status !== "authenticated" || !supported || permission !== "default" || dismissed) return null

  return (
    <div className="flex items-center justify-between bg-brand-primary px-6 py-2 text-sm text-white">
      <span>{t("message")}</span>
      <div className="flex items-center gap-3">
        <button
          onClick={enable}
          className="cursor-pointer rounded bg-white px-3 py-1 text-xs font-medium text-brand-primary transition-colors hover:bg-brand-pale focus:outline-none focus:ring-2 focus:ring-white/50"
        >
          {t("enable")}
        </button>
        <button
          onClick={() => setDismissed(true)}
          aria-label={t("dismiss")}
          className="cursor-pointer text-brand-light transition-colors hover:text-white focus:outline-none focus:ring-2 focus:ring-white/50 rounded"
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>
    </div>
  )
}
