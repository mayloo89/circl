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
  const [activeIdx, setActiveIdx] = useState(-1)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const listboxId = "interests-listbox"

  useEffect(() => {
    const id = setTimeout(async () => {
      if (!query.trim() || !token) {
        setSuggestions([])
        setActiveIdx(-1)
        return
      }
      try {
        const res = await fetch(
          `${API_URL}/profiles/interests?q=${encodeURIComponent(query)}&limit=8`,
          { headers: { Authorization: `Bearer ${token}` } },
        )
        if (!res.ok) return
        const data: { name: string }[] = await res.json()
        setSuggestions(data.map((d) => d.name).filter((n) => !interests.includes(n)))
        setActiveIdx(-1)
      } catch { /* ignore */ }
    }, 250)
    return () => clearTimeout(id)
  }, [query, token, interests])

  function addInterest(name: string) {
    if (interests.includes(name) || interests.length >= MAX_INTERESTS) return
    setInterests((prev) => [...prev, name])
    setQuery("")
    setSuggestions([])
    setActiveIdx(-1)
  }

  function removeInterest(name: string) {
    setInterests((prev) => prev.filter((i) => i !== name))
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "ArrowDown") {
      if (suggestions.length === 0) return
      e.preventDefault()
      setActiveIdx((i) => Math.min(i + 1, suggestions.length - 1))
    } else if (e.key === "ArrowUp") {
      if (suggestions.length === 0) return
      e.preventDefault()
      setActiveIdx((i) => Math.max(i - 1, -1))
    } else if (e.key === "Escape") {
      setSuggestions([])
      setActiveIdx(-1)
    } else if (e.key === "Enter" || e.key === ",") {
      e.preventDefault()
      if (activeIdx >= 0 && suggestions[activeIdx]) {
        addInterest(suggestions[activeIdx])
        return
      }
      const tag = query.trim().toLowerCase().replace(/,/g, "")
      if (tag) addInterest(tag)
    }
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
        <h1 className="text-2xl font-bold text-foreground">{t("interests.title")}</h1>
        <p className="text-sm text-gray-400">{t("interests.subtitle")}</p>
      </div>

      <div className="relative">
        <input
          type="text"
          role="combobox"
          aria-expanded={suggestions.length > 0}
          aria-autocomplete="list"
          aria-controls={listboxId}
          aria-activedescendant={activeIdx >= 0 ? `interest-option-${activeIdx}` : undefined}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={t("interests.placeholder")}
          disabled={interests.length >= MAX_INTERESTS}
          className="w-full rounded-lg border border-gray-700 bg-gray-900 px-4 py-3 text-foreground placeholder-gray-600 focus:border-brand-hover focus:outline-none focus:ring-1 focus:ring-brand-hover disabled:opacity-50"
        />
        {suggestions.length > 0 && (
          <ul
            id={listboxId}
            role="listbox"
            className="absolute z-10 mt-1 w-full overflow-hidden rounded-lg border border-gray-700 bg-gray-800 shadow-xl"
          >
            {suggestions.map((s, idx) => (
              <li
                key={s}
                id={`interest-option-${idx}`}
                role="option"
                aria-selected={idx === activeIdx}
              >
                <button
                  type="button"
                  onClick={() => addInterest(s)}
                  className={`w-full px-4 py-2.5 text-left text-sm text-gray-200 ${idx === activeIdx ? "bg-gray-600" : "hover:bg-gray-700"}`}
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
              className="flex items-center gap-1.5 rounded-full bg-brand-primary/15 px-3 py-1.5 text-sm font-medium text-brand-strong ring-1 ring-brand-primary/30"
            >
              {tag}
              <button
                type="button"
                onClick={() => removeInterest(tag)}
                aria-label={t("interests.removeInterest", { tag })}
                className="text-brand-muted hover:text-foreground"
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
