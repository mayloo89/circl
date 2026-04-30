"use client"

import { useRouter } from "@/i18n/navigation"
import { useSession } from "next-auth/react"
import { useProfileContext } from "@/contexts/ProfileContext"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export function useOnboardingSkip() {
  const router = useRouter()
  const { data: session } = useSession()
  const { refresh } = useProfileContext()

  return async function skipStep() {
    const token = session?.accessToken
    if (token) {
      await fetch(`${API_URL}/profiles/me`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ mark_onboarded: true }),
      }).catch(() => {})
    }
    await refresh()
    router.replace("/")
  }
}
