"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { usePathname } from "next/navigation"
import { useTranslations } from "next-intl"
import { useRouter } from "@/i18n/navigation"
import { useSession } from "next-auth/react"
import { useProfileContext } from "@/contexts/ProfileContext"
import { OnboardingContext } from "./context"

const STEPS = ["photo", "bio", "interests", "location"] as const
const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

function stepFromPathname(pathname: string): number {
  for (let i = 0; i < STEPS.length; i++) {
    if (pathname.includes(`/onboarding/${STEPS[i]}`)) return i
  }
  return 0
}

export default function OnboardingLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname()
  const router = useRouter()
  const { data: session } = useSession()
  const t = useTranslations("onboarding")
  const { refresh } = useProfileContext()
  const [dismissForever, setDismissForever] = useState(false)
  const tokenRef = useRef(session?.accessToken)

  useEffect(() => { tokenRef.current = session?.accessToken }, [session?.accessToken])

  const currentStep = stepFromPathname(pathname)
  const total = STEPS.length

  async function markOnboarded() {
    const token = tokenRef.current
    if (token) {
      await fetch(`${API_URL}/profiles/me`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ mark_onboarded: true }),
      }).catch(() => {})
    }
    await refresh()
  }

  async function handleSkipAll() {
    await markOnboarded()
    router.replace("/")
  }

  const skipStep = useCallback(async (nextPath: string) => {
    if (dismissForever) {
      await markOnboarded()
      router.replace("/")
    } else {
      router.push(nextPath)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dismissForever, router])

  return (
    <OnboardingContext.Provider value={{ skipStep }}>
      <div className="flex min-h-dvh flex-col bg-gray-950">
        {/* Header */}
        <header className="flex items-center justify-between px-5 py-4">
          {/* Step dots */}
          <div className="flex items-center gap-2" aria-label={t("stepIndicator", { current: currentStep + 1, total })}>
            {STEPS.map((_, i) => (
              <span
                key={i}
                className={`h-2 rounded-full transition-all duration-300 ${
                  i === currentStep
                    ? "w-6 bg-brand-accent"
                    : i < currentStep
                    ? "w-2 bg-brand-primary"
                    : "w-2 bg-gray-700"
                }`}
              />
            ))}
          </div>

          <div className="flex flex-col items-end gap-1.5">
            <button
              type="button"
              onClick={handleSkipAll}
              className="text-sm text-gray-400 hover:text-gray-200 transition-colors"
            >
              {t("skipAll")}
            </button>
            <label className="flex items-center gap-1.5 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={dismissForever}
                onChange={(e) => setDismissForever(e.target.checked)}
                className="h-3 w-3 rounded border-gray-600 bg-gray-800 accent-brand-hover"
              />
              <span className="text-xs text-gray-500">{t("neverShow")}</span>
            </label>
          </div>
        </header>

        {/* Step content */}
        <main className="flex flex-1 flex-col items-center px-5 pb-10 pt-6">
          <div className="w-full max-w-md">
            {children}
          </div>
        </main>
      </div>
    </OnboardingContext.Provider>
  )
}
