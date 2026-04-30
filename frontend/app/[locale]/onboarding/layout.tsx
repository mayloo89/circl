"use client"

import { useState } from "react"
import { usePathname } from "next/navigation"
import { useTranslations } from "next-intl"
import { useRouter } from "@/i18n/navigation"
import { useSession } from "next-auth/react"
import { useProfileContext } from "@/contexts/ProfileContext"

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

  const currentStep = stepFromPathname(pathname)
  const total = STEPS.length

  return (
    <div className="flex min-h-dvh flex-col bg-gray-950">
      <header className="flex items-center justify-between px-5 py-4">
        <span className="text-sm font-medium text-gray-400" aria-label={t("stepIndicator", { current: currentStep + 1, total })}>
          {currentStep + 1} / {total}
        </span>

        <button
          type="button"
          onClick={handleSkipAll}
          disabled={skipping}
          className="text-sm text-gray-400 hover:text-gray-200 transition-colors disabled:opacity-50"
        >
          {t("skipAll")}
        </button>
      </header>

      <main className="flex flex-1 flex-col items-center px-5 pb-10 pt-6">
        <div className="w-full max-w-md">
          {children}
        </div>
      </main>
    </div>
  )
}
