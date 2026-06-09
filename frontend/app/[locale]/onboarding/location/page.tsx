"use client"

import { useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { useRouter } from "@/i18n/navigation"
import Button from "@/components/ui/Button"
import { useProfileContext } from "@/contexts/ProfileContext"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface LocationSuggestion {
  label: string
  lat: number
  lng: number
}

interface PhotonFeature {
  geometry: { coordinates: [number, number] }
  properties: { name?: string; city?: string; state?: string; country?: string }
}

export default function OnboardingLocationPage() {
  const { data: session } = useSession()
  const router = useRouter()
  const t = useTranslations("onboarding")
  const token = session?.accessToken
  const { profile, refresh } = useProfileContext()

  const existingText = profile?.location_text ?? ""
  const [locationText, setLocationText] = useState(existingText)
  const [locationLat, setLocationLat] = useState<number | null>(null)
  const [locationLng, setLocationLng] = useState<number | null>(null)
  const [query, setQuery] = useState(existingText)
  const [suggestions, setSuggestions] = useState<LocationSuggestion[]>([])
  const [activeIdx, setActiveIdx] = useState(-1)
  const [locating, setLocating] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const listboxId = "location-listbox"

  // Fetch coordinates for the pre-existing location (text comes from context, coords need API)
  useEffect(() => {
    if (!token || !existingText) return
    fetch(`${API_URL}/profiles/me`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => r.ok ? r.json() : null)
      .then((d) => {
        if (d?.latitude != null) setLocationLat(d.latitude)
        if (d?.longitude != null) setLocationLng(d.longitude)
      })
      .catch(() => {})
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token])

  useEffect(() => {
    if (query.length < 2) { setSuggestions([]); return }
    const id = setTimeout(async () => {
      try {
        const res = await fetch(
          `https://photon.komoot.io/api/?q=${encodeURIComponent(query)}&limit=5`,
          { headers: { "Accept-Language": navigator.language ?? "en" } },
        )
        if (!res.ok) return
        const data = await res.json()
        const seen = new Set<string>()
        const results = (data.features as PhotonFeature[]).flatMap((f) => {
          const p = f.properties
          const parts = [p.name, p.city ?? p.state, p.country].filter(Boolean)
          const label = parts.join(", ")
          const key = `${label}-${f.geometry.coordinates[1]},${f.geometry.coordinates[0]}`
          if (seen.has(key)) return []
          seen.add(key)
          return [{ label, lat: f.geometry.coordinates[1], lng: f.geometry.coordinates[0] }]
        })
        setSuggestions(results)
      } catch {
        setSuggestions([])
      }
    }, 350)
    return () => clearTimeout(id)
  }, [query])

  useEffect(() => { setActiveIdx(-1) }, [suggestions])

  function selectSuggestion(s: LocationSuggestion) {
    setLocationText(s.label)
    setQuery(s.label)
    setLocationLat(s.lat)
    setLocationLng(s.lng)
    setSuggestions([])
    setActiveIdx(-1)
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
    } else if (e.key === "Enter") {
      if (activeIdx >= 0 && suggestions[activeIdx]) {
        e.preventDefault()
        selectSuggestion(suggestions[activeIdx])
      }
    }
  }

  function handleGeolocate() {
    if (!navigator.geolocation) return
    setLocating(true)
    navigator.geolocation.getCurrentPosition(
      async (pos) => {
        try {
          const res = await fetch(
            `https://photon.komoot.io/reverse?lon=${pos.coords.longitude}&lat=${pos.coords.latitude}&limit=1`,
          )
          if (res.ok) {
            const data = await res.json()
            const f: PhotonFeature = data.features?.[0]
            if (f) {
              const p = f.properties
              const parts = [p.name, p.city ?? p.state, p.country].filter(Boolean)
              const label = parts.join(", ")
              setLocationText(label)
              setQuery(label)
              setLocationLat(f.geometry.coordinates[1])
              setLocationLng(f.geometry.coordinates[0])
              setSuggestions([])
            }
          }
        } catch { /* ignore */ }
        setLocating(false)
      },
      () => setLocating(false),
    )
  }

  async function handleFinish() {
    if (!token) { router.replace("/"); return }
    setSaving(true)
    setError("")
    try {
      // If the user typed a location but never selected from autocomplete,
      // geocode it now so we get coordinates for distance calculations.
      let finalLat = locationLat
      let finalLng = locationLng
      if (locationText.trim() && (finalLat === null || finalLng === null)) {
        try {
          const geo = await fetch(
            `https://photon.komoot.io/api/?q=${encodeURIComponent(locationText)}&limit=1`,
            { headers: { "Accept-Language": navigator.language ?? "en" } },
          )
          if (geo.ok) {
            const data = await geo.json()
            const f: PhotonFeature | undefined = data.features?.[0]
            if (f) {
              finalLat = f.geometry.coordinates[1]
              finalLng = f.geometry.coordinates[0]
            }
          }
        } catch { /* proceed without coordinates */ }
      }

      const profileRes = await fetch(`${API_URL}/profiles/me`, { headers: { Authorization: `Bearer ${token}` } })
      if (!profileRes.ok) { setError(t("saveError")); return }
      const current = await profileRes.json()
      const body: Record<string, unknown> = {
        ...current,
        mark_onboarded: true,
      }
      if (locationText) {
        body.location_text = locationText
        body.latitude = finalLat
        body.longitude = finalLng
      }
      const res = await fetch(`${API_URL}/profiles/me`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify(body),
      })
      if (!res.ok) { setError(t("saveError")); return }
      await refresh()
      router.replace("/")
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex flex-col gap-8">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold text-foreground">{t("location.title")}</h1>
        <p className="text-sm text-gray-400">{t("location.subtitle")}</p>
      </div>

      <div className="space-y-3">
        <div className="relative">
          <input
            type="text"
            role="combobox"
            aria-expanded={suggestions.length > 0}
            aria-autocomplete="list"
            aria-controls={listboxId}
            aria-activedescendant={activeIdx >= 0 ? `location-option-${activeIdx}` : undefined}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setLocationText(e.target.value)
              setLocationLat(null)
              setLocationLng(null)
            }}
            onKeyDown={handleKeyDown}
            onBlur={() => setTimeout(() => setSuggestions([]), 150)}
            placeholder={t("location.placeholder")}
            autoComplete="off"
            className="w-full rounded-lg border border-gray-700 bg-gray-900 px-4 py-3 text-foreground placeholder-gray-600 focus:border-brand-hover focus:outline-none focus:ring-1 focus:ring-brand-hover"
          />
          {suggestions.length > 0 && (
            <ul
              id={listboxId}
              role="listbox"
              className="absolute z-10 mt-1 w-full overflow-hidden rounded-lg border border-gray-700 bg-gray-800 shadow-xl"
            >
              {suggestions.map((s, idx) => (
                <li
                  key={`${s.label}-${s.lat},${s.lng}`}
                  id={`location-option-${idx}`}
                  role="option"
                  aria-selected={idx === activeIdx}
                >
                  <button
                    type="button"
                    onClick={() => selectSuggestion(s)}
                    className={`w-full px-4 py-2.5 text-left text-sm text-gray-200 ${idx === activeIdx ? "bg-gray-600" : "hover:bg-gray-700"}`}
                  >
                    {s.label}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>

        <button
          type="button"
          onClick={handleGeolocate}
          disabled={locating}
          className="flex items-center gap-2 text-sm text-brand-subtle hover:text-foreground disabled:opacity-50"
        >
          <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" d="M15 10.5a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
            <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 10.5c0 7.142-7.5 11.25-7.5 11.25S4.5 17.642 4.5 10.5a7.5 7.5 0 1 1 15 0Z" />
          </svg>
          {locating ? t("location.detecting") : t("location.useMyLocation")}
        </button>
      </div>

      {error && <p className="text-sm text-red-400">{error}</p>}

      <div className="flex flex-col gap-3">
        <Button
          variant="accent"
          className="w-full"
          onClick={handleFinish}
          loading={saving}
          disabled={saving}
        >
          {locationText.trim() ? t("finish") : t("finishSkip")}
        </Button>
      </div>
    </div>
  )
}
