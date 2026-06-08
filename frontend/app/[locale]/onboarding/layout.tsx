"use client"

import { useState } from "react"
import { usePathname } from "next/navigation"
import { useTranslations } from "next-intl"
import { useRouter } from "@/i18n/navigation"
import { useSession } from "next-auth/react"
import { useProfileContext } from "@/contexts/ProfileContext"
import { ONBOARDING_STEPS, incompleteSteps } from "@/lib/onboardingSteps"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export default function OnboardingLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname()
  const router = useRouter()
  const { data: session } = useSession()
  const t = useTranslations("onboarding")
  const { profile, refresh } = useProfileContext()
  const [skipping, setSkipping] = useState(false)

  async function handleSkipAll() {
    if (skipping) return
    setSkipping(true)
    const token = session?.accessToken
    if (token) {
      try {
        await fetch(`${API_URL}/profiles/me`, {
          method: "PUT",
          headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
          body: JSON.stringify({ mark_onboarded: true }),
        })
        await refresh()
      } catch {}
    }
    router.replace("/")
  }

  // Pending steps = what was incomplete when the user entered the wizard.
  // We fall back to all steps if profile hasn't loaded yet to avoid flicker.
  const pending = profile ? incompleteSteps(profile) : ONBOARDING_STEPS
  const currentStepName = ONBOARDING_STEPS.find((s) => pathname.includes(`/onboarding/${s}`))
  const currentIdx = currentStepName ? pending.indexOf(currentStepName) : 0
  const total = pending.length || 1
  const current = currentIdx >= 0 ? currentIdx + 1 : 1

  return (
    <div className="flex min-h-dvh flex-col bg-gray-950">
      <header className="px-5 pt-4 pb-0">
        <div className="flex items-center justify-between mb-3">
          <span className="sr-only" aria-live="polite">
            {t("stepIndicator", { current, total })}
          </span>
          <span className="text-xs font-medium text-gray-500" aria-hidden="true">
            {current} / {total}
          </span>
          <button
            type="button"
            onClick={handleSkipAll}
            disabled={skipping}
            className="flex min-h-[36px] items-center rounded px-2 text-sm text-gray-400 hover:text-gray-200 transition-colors disabled:opacity-50"
          >
            {t("skipAll")}
          </button>
        </div>
        <div className="h-1 w-full overflow-hidden rounded-full bg-gray-800" role="progressbar" aria-valuenow={current} aria-valuemin={1} aria-valuemax={total}>
          <div
            className="h-full rounded-full bg-brand-primary transition-all duration-500 ease-out"
            style={{ width: `${(current / total) * 100}%` }}
          />
        </div>
      </header>

      <main className="flex flex-1 flex-col items-center px-5 pb-10 pt-6">
        <div className="w-full max-w-md">
          {children}
        </div>
      </main>
    </div>
  )
}
