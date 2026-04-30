"use client"

import { useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { useRouter } from "@/i18n/navigation"
import Button from "@/components/ui/Button"
import { useOnboardingContext } from "../context"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
const MAX_BIO = 280

export default function OnboardingBioPage() {
  const { data: session } = useSession()
  const router = useRouter()
  const t = useTranslations("onboarding")
  const token = session?.accessToken

  const [bio, setBio] = useState("")
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const { skipStep } = useOnboardingContext()

  useEffect(() => {
    if (!token) return
    fetch(`${API_URL}/profiles/me`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => r.ok ? r.json() : null)
      .then((d) => { if (d?.bio) setBio(d.bio) })
      .catch(() => {})
  }, [token])

  async function handleContinue() {
    if (!token) { await skipStep("/onboarding/interests"); return }
    if (!bio.trim()) { await skipStep("/onboarding/interests"); return }
    setSaving(true)
    setError("")
    try {
      const profileRes = await fetch(`${API_URL}/profiles/me`, { headers: { Authorization: `Bearer ${token}` } })
      if (!profileRes.ok) { setError(t("saveError")); return }
      const current = await profileRes.json()
      const res = await fetch(`${API_URL}/profiles/me`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ ...current, bio }),
      })
      if (!res.ok) { setError(t("saveError")); return }
      router.push("/onboarding/interests")
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex flex-col gap-8">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold text-white">{t("bio.title")}</h1>
        <p className="text-sm text-gray-400">{t("bio.subtitle")}</p>
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label htmlFor="bio" className="text-sm font-medium text-gray-300">{t("bio.label")}</label>
          <span className={`text-xs ${bio.length > MAX_BIO ? "text-red-400" : "text-gray-500"}`}>
            {bio.length}/{MAX_BIO}
          </span>
        </div>
        <textarea
          id="bio"
          value={bio}
          onChange={(e) => setBio(e.target.value)}
          rows={5}
          maxLength={MAX_BIO}
          placeholder={t("bio.placeholder")}
          className="w-full resize-none rounded-lg border border-gray-700 bg-gray-900 px-4 py-3 text-white placeholder-gray-600 focus:border-brand-hover focus:outline-none focus:ring-1 focus:ring-brand-hover"
        />
      </div>

      {error && <p className="text-sm text-red-400">{error}</p>}

      <div className="flex flex-col gap-3">
        <Button
          variant="accent"
          className="w-full"
          onClick={handleContinue}
          loading={saving}
          disabled={saving}
        >
          {bio.trim() ? t("continue") : t("skip")}
        </Button>
      </div>
    </div>
  )
}
