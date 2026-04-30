"use client"

import { useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { useRouter } from "@/i18n/navigation"
import { useProfileContext } from "@/contexts/ProfileContext"
import { nextStepAfter } from "@/lib/onboardingSteps"
import Button from "@/components/ui/Button"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
const MAX_INTERESTS = 20

export default function OnboardingInterestsPage() {
  const { data: session } = useSession()
  const router = useRouter()
  const t = useTranslations("onboarding")
  const token = session?.accessToken
  const { profile, refresh } = useProfileContext()

  const [interests, setInterests] = useState<string[]>(profile?.interests ?? [])
  const [query, setQuery] = useState("")
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!query.trim() || !token) { setSuggestions([]); return }
    const id = setTimeout(async () => {
      try {
        const res = await fetch(
          `${API_URL}/profiles/interests?q=${encodeURIComponent(query)}&limit=8`,
          { headers: { Authorization: `Bearer ${token}` } },
        )
        if (!res.ok) return
        const data: { name: string }[] = await res.json()
        setSuggestions(data.map((d) => d.name).filter((n) => !interests.includes(n)))
      } catch { /* ignore */ }
    }, 250)
    return () => clearTimeout(id)
  }, [query, token, interests])

  function addInterest(name: string) {
    if (interests.includes(name) || interests.length >= MAX_INTERESTS) return
    setInterests((prev) => [...prev, name])
    setQuery("")
    setSuggestions([])
  }

  function removeInterest(name: string) {
    setInterests((prev) => prev.filter((i) => i !== name))
  }

  async function handleContinue() {
    if (!token) return
    const next = nextStepAfter(profile!, "interests")

    if (interests.length > 0) {
      setSaving(true)
      setError("")
      try {
        const profileRes = await fetch(`${API_URL}/profiles/me`, { headers: { Authorization: `Bearer ${token}` } })
        if (!profileRes.ok) { setError(t("saveError")); return }
        const current = await profileRes.json()
        const res = await fetch(`${API_URL}/profiles/me`, {
          method: "PUT",
          headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
          body: JSON.stringify({ ...current, interests }),
        })
        if (!res.ok) { setError(t("saveError")); return }
      } finally {
        setSaving(false)
      }
    }

    if (next === "/") {
      setSaving(true)
      try {
        await fetch(`${API_URL}/profiles/me`, {
          method: "PUT",
          headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
          body: JSON.stringify({ mark_onboarded: true }),
        })
        await refresh()
        router.replace("/")
      } finally {
        setSaving(false)
      }
    } else {
      router.push(next)
    }
  }

  return (
    <div className="flex flex-col gap-8">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold text-white">{t("interests.title")}</h1>
        <p className="text-sm text-gray-400">{t("interests.subtitle")}</p>
      </div>

      <div className="relative">
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key !== "Enter" && e.key !== ",") return
            e.preventDefault()
            const tag = query.trim().toLowerCase().replace(/,/g, "")
            if (tag) addInterest(tag)
          }}
          placeholder={t("interests.placeholder")}
          disabled={interests.length >= MAX_INTERESTS}
          className="w-full rounded-lg border border-gray-700 bg-gray-900 px-4 py-3 text-white placeholder-gray-600 focus:border-brand-hover focus:outline-none focus:ring-1 focus:ring-brand-hover disabled:opacity-50"
        />
        {suggestions.length > 0 && (
          <ul className="absolute z-10 mt-1 w-full overflow-hidden rounded-lg border border-gray-700 bg-gray-800 shadow-xl">
            {suggestions.map((s) => (
              <li key={s}>
                <button
                  type="button"
                  onClick={() => addInterest(s)}
                  className="w-full px-4 py-2.5 text-left text-sm text-gray-200 hover:bg-gray-700"
                >
                  {s}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {interests.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {interests.map((tag) => (
            <span
              key={tag}
              className="flex items-center gap-1.5 rounded-full bg-brand-wash/50 px-3 py-1.5 text-sm text-brand-subtle ring-1 ring-brand-strong/60"
            >
              {tag}
              <button
                type="button"
                onClick={() => removeInterest(tag)}
                aria-label={`Remove ${tag}`}
                className="text-brand-muted hover:text-white"
              >
                ×
              </button>
            </span>
          ))}
        </div>
      )}

      {error && <p className="text-sm text-red-400">{error}</p>}

      <div className="flex flex-col gap-3">
        <Button
          variant="accent"
          className="w-full"
          onClick={handleContinue}
          loading={saving}
          disabled={saving}
        >
          {interests.length > 0 ? t("continue") : t("skip")}
        </Button>
      </div>
    </div>
  )
}
