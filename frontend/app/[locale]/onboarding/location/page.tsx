"use client"

import { useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { useRouter } from "@/i18n/navigation"
import Button from "@/components/ui/Button"

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

  const [locationText, setLocationText] = useState("")
  const [locationLat, setLocationLat] = useState<number | null>(null)
  const [locationLng, setLocationLng] = useState<number | null>(null)
  const [query, setQuery] = useState("")
  const [suggestions, setSuggestions] = useState<LocationSuggestion[]>([])
  const [locating, setLocating] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!token) return
    fetch(`${API_URL}/profiles/me`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => r.ok ? r.json() : null)
      .then((d) => {
        if (d?.location_text) {
          setLocationText(d.location_text)
          setQuery(d.location_text)
          setLocationLat(d.latitude ?? null)
          setLocationLng(d.longitude ?? null)
        }
      })
      .catch(() => {})
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
      const profileRes = await fetch(`${API_URL}/profiles/me`, { headers: { Authorization: `Bearer ${token}` } })
      if (!profileRes.ok) { setError(t("saveError")); return }
      const current = await profileRes.json()
      const body: Record<string, unknown> = {
        ...current,
        mark_onboarded: true,
      }
      if (locationText) {
        body.location_text = locationText
        body.latitude = locationLat
        body.longitude = locationLng
      }
      const res = await fetch(`${API_URL}/profiles/me`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify(body),
      })
      if (!res.ok) { setError(t("saveError")); return }
      router.replace("/")
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex flex-col gap-8">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold text-white">{t("location.title")}</h1>
        <p className="text-sm text-gray-400">{t("location.subtitle")}</p>
      </div>

      {/* Location input */}
      <div className="space-y-3">
        <div className="relative">
          <input
            type="text"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setLocationText(e.target.value)
              setLocationLat(null)
              setLocationLng(null)
            }}
            onBlur={() => setTimeout(() => setSuggestions([]), 150)}
            placeholder={t("location.placeholder")}
            autoComplete="off"
            className="w-full rounded-lg border border-gray-700 bg-gray-900 px-4 py-3 text-white placeholder-gray-600 focus:border-brand-hover focus:outline-none focus:ring-1 focus:ring-brand-hover"
          />
          {suggestions.length > 0 && (
            <ul className="absolute z-10 mt-1 w-full overflow-hidden rounded-lg border border-gray-700 bg-gray-800 shadow-xl">
              {suggestions.map((s) => (
                <li key={`${s.label}-${s.lat},${s.lng}`}>
                  <button
                    type="button"
                    onClick={() => {
                      setLocationText(s.label)
                      setQuery(s.label)
                      setLocationLat(s.lat)
                      setLocationLng(s.lng)
                      setSuggestions([])
                    }}
                    className="w-full px-4 py-2.5 text-left text-sm text-gray-200 hover:bg-gray-700"
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
          className="flex items-center gap-2 text-sm text-brand-subtle hover:text-white disabled:opacity-50"
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
