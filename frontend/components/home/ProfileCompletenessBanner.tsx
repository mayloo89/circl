"use client"

import { useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { Link } from "@/i18n/navigation"
import { computeCompleteness, type ProfileFieldsForCompleteness, type MissingField } from "@/lib/profileCompleteness"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
const DISMISSED_KEY = "profile-banner-dismissed"

const MISSING_LINK: Record<MissingField, string> = {
  avatar: "/profile",
  bio: "/profile",
  interests: "/profile",
  birthdate: "/profile",
  location: "/profile",
}

export default function ProfileCompletenessBanner() {
  const t = useTranslations("home")
  const { data: session, status } = useSession()
  const [profile, setProfile] = useState<ProfileFieldsForCompleteness | null>(null)
  const [dismissed, setDismissed] = useState(() => {
    if (typeof window === "undefined") return false
    return sessionStorage.getItem(DISMISSED_KEY) === "1"
  })

  useEffect(() => {
    if (status !== "authenticated" || !session?.accessToken) return
    const token = session.accessToken
    let cancelled = false
    fetch(`${API_URL}/profiles/me`, { headers: { Authorization: `Bearer ${token}` } })
      .then((res) => (res.ok ? res.json() : Promise.reject()))
      .then((data) => {
        if (!cancelled) {
          setProfile({
            avatar_url: data.avatar_url ?? "",
            bio: data.bio ?? "",
            interests: data.interests ?? [],
            date_of_birth: data.date_of_birth ?? "",
            location_text: data.location_text ?? "",
          })
        }
      })
      .catch(() => {})
    return () => { cancelled = true }
  }, [status, session?.accessToken])

  if (dismissed || !profile) return null

  const { percent, missing } = computeCompleteness(profile)
  if (percent >= 100) return null

  const firstMissing = missing[0]
  const setupHref = firstMissing ? MISSING_LINK[firstMissing] : "/profile"

  function dismiss() {
    sessionStorage.setItem(DISMISSED_KEY, "1")
    setDismissed(true)
  }

  return (
    <div className="rounded-card bg-gray-900 dark:bg-white/[0.04] shadow-card ring-1 ring-brand-primary/20 dark:ring-white/[0.08] p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex-1 min-w-0">
          <p className="text-sm font-semibold text-foreground">
            {t("completeProfile")} — {percent}%
          </p>
          <p className="mt-0.5 text-xs text-gray-400">{t("completeProfileDesc")}</p>

          <div className="mt-3 h-1.5 w-full overflow-hidden rounded-full bg-gray-700">
            <div
              className="h-full rounded-full bg-brand-primary transition-all duration-500"
              style={{ width: `${percent}%` }}
              role="progressbar"
              aria-valuenow={percent}
              aria-valuemin={0}
              aria-valuemax={100}
            />
          </div>

          {missing.length > 0 && (
            <ul className="mt-2 space-y-0.5">
              {missing.map((field) => (
                <li key={field} className="text-xs text-gray-500">
                  · {t(`missing_${field}` as Parameters<typeof t>[0])}
                </li>
              ))}
            </ul>
          )}
        </div>

        <button
          onClick={dismiss}
          aria-label={t("dismiss")}
          className="flex-none cursor-pointer rounded p-3.5 text-gray-600 transition-colors hover:text-gray-400 focus:outline-none focus:ring-2 focus:ring-brand-hover"
        >
          <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>

      <div className="mt-3">
        <Link
          href={setupHref}
          className="inline-flex items-center gap-1 rounded bg-brand-primary px-3 py-1.5 text-xs font-medium text-foreground hover:bg-brand-hover transition-colors"
        >
          {t("continueSetup")}
          <svg className="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <polyline points="9 18 15 12 9 6" />
          </svg>
        </Link>
      </div>
    </div>
  )
}
