"use client"

import Image from "next/image"
import { useSession } from "next-auth/react"
import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslations } from "next-intl"

import { Link } from "@/i18n/navigation"

import Avatar from "@/components/ui/Avatar"
import BottomSheet from "@/components/ui/BottomSheet"
import Button from "@/components/ui/Button"
import RangeSlider from "@/components/ui/RangeSlider"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
const PAGE_SIZE = 12
const AGE_MIN = 18
const AGE_MAX = 99
const DIST_MAX = 500

interface BrowseProfile {
  id: string
  user_id: string
  username: string
  display_name: string
  avatar_url: string
  age: number | null
  gender: string
  location_text: string
  distance_km: number | null
  first_photo_url: string
  interests: string[]
}

interface BrowsePage {
  profiles: BrowseProfile[]
  next_cursor: string
}

interface Preferences {
  min_age: number | null
  max_age: number | null
  max_distance_km: number | null
  gender_preference: string[]
}

const GENDER_OPTIONS = ["Man", "Woman", "Non-binary", "Other"]

function ProfileCardSkeleton() {
  return (
    <div className="rounded-card bg-gray-900 shadow-card ring-1 ring-gray-800 overflow-hidden">
      <Skeleton className="aspect-[4/5] w-full rounded-none" />
      <div className="p-4 space-y-2">
        <Skeleton className="h-4 w-28" />
        <Skeleton className="h-3 w-20" />
      </div>
    </div>
  )
}

function formatDistance(km: number | null): string | null {
  if (km === null) return null
  if (km < 1) return "< 1 km away"
  return `${Math.round(km)} km away`
}

function SendRequestButton({ userID, token }: { userID: string; token: string }) {
  const t = useTranslations("browse")
  const [status, setStatus] = useState<"idle" | "loading" | "sent" | "error">("idle")

  async function handleSend(e: React.MouseEvent) {
    e.preventDefault()
    e.stopPropagation()
    setStatus("loading")
    try {
      const res = await fetch(`${API_URL}/contacts`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ addressee_id: userID }),
      })
      setStatus(res.ok || res.status === 409 ? "sent" : "error")
    } catch {
      setStatus("error")
    }
  }

  if (status === "sent") {
    return <span className="text-xs text-brand-muted font-medium">{t("requestSent")}</span>
  }

  return (
    <Button
      variant="accent"
      size="sm"
      onClick={handleSend}
      loading={status === "loading"}
    >
      {status === "error" ? t("retry") : t("addContact")}
    </Button>
  )
}

function ProfileCard({ profile, token }: { profile: BrowseProfile; token: string }) {
  const heroURL = profile.avatar_url
  const distance = formatDistance(profile.distance_km)

  return (
    <Link
      href={`/profile/${profile.username}`}
      className="group rounded-card bg-gray-900 shadow-card ring-1 ring-gray-800 overflow-hidden flex flex-col hover:shadow-card-hover hover:ring-brand-strong transition-shadow"
    >
      {heroURL ? (
        <div className="relative aspect-[4/5] w-full overflow-hidden bg-gray-800">
          <Image
            src={heroURL}
            alt={profile.display_name}
            fill
            className="object-cover group-hover:scale-105 transition-transform duration-300"
            sizes="(max-width: 640px) 50vw, (max-width: 1024px) 33vw, 25vw"
          />
        </div>
      ) : (
        <div className="aspect-[4/5] w-full bg-gray-800 flex items-center justify-center">
          <Avatar src="" name={profile.display_name} size="xl" />
        </div>
      )}

      <div className="p-4 flex flex-col gap-3 flex-1">
        <div>
          <p className="font-semibold text-white truncate">
            {profile.display_name}
            {profile.age !== null && (
              <span className="text-gray-400 font-normal">, {profile.age}</span>
            )}
          </p>
          <div className="mt-0.5 flex flex-wrap items-center gap-x-2 text-xs text-gray-500">
            {profile.gender && <span>{profile.gender}</span>}
            {distance && (
              <>
                {profile.gender && <span>·</span>}
                <span>{distance}</span>
              </>
            )}
          </div>
        </div>

        {profile.interests.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {profile.interests.slice(0, 3).map((tag) => (
              <span
                key={tag}
                className="rounded-full bg-brand-wash/50 px-2 py-0.5 text-xs text-brand-subtle ring-1 ring-brand-strong/60"
              >
                {tag}
              </span>
            ))}
            {profile.interests.length > 3 && (
              <span className="rounded-full bg-gray-800 px-2 py-0.5 text-xs text-gray-500">
                +{profile.interests.length - 3}
              </span>
            )}
          </div>
        )}

        {/* Stop propagation so clicking the button doesn't navigate */}
        <div className="mt-auto pt-1" onClick={(e) => e.preventDefault()}>
          <SendRequestButton userID={profile.user_id} token={token} />
        </div>
      </div>
    </Link>
  )
}

interface FilterPanelProps {
  prefs: Preferences
  sortByDistance: boolean
  selectedInterests: string[]
  token: string
  onApply: (p: Preferences, sortByDistance: boolean, interests: string[]) => void
  saving: boolean
}

function FilterPanel({ prefs, sortByDistance, selectedInterests, token, onApply, saving }: FilterPanelProps) {
  const t = useTranslations("browse")
  const [draft, setDraft] = useState<Preferences>(prefs)
  const [draftSort, setDraftSort] = useState(sortByDistance)
  const [draftInterests, setDraftInterests] = useState<string[]>(selectedInterests)
  const [interestQuery, setInterestQuery] = useState("")
  const [interestSuggestions, setInterestSuggestions] = useState<string[]>([])
  const tokenRef = useRef(token)

  useEffect(() => { setDraft(prefs) }, [prefs])
  useEffect(() => { setDraftSort(sortByDistance) }, [sortByDistance])
  useEffect(() => { setDraftInterests(selectedInterests) }, [selectedInterests])
  useEffect(() => { tokenRef.current = token }, [token])

  useEffect(() => {
    if (interestQuery.length < 1) { setInterestSuggestions([]); return }
    const id = setTimeout(async () => {
      try {
        const res = await fetch(`${API_URL}/profiles/interests?q=${encodeURIComponent(interestQuery)}&limit=8`, {
          headers: { Authorization: `Bearer ${tokenRef.current}` },
        })
        if (!res.ok) return
        const data: { name: string }[] = await res.json()
        setInterestSuggestions(data.map((d) => d.name).filter((n) => !draftInterests.includes(n)))
      } catch { /* ignore */ }
    }, 250)
    return () => clearTimeout(id)
  }, [interestQuery, draftInterests])

  function addInterest(name: string) {
    setDraftInterests((prev) => prev.includes(name) ? prev : [...prev, name])
    setInterestQuery("")
    setInterestSuggestions([])
  }

  function removeInterest(name: string) {
    setDraftInterests((prev) => prev.filter((i) => i !== name))
  }

  function toggleGender(g: string) {
    setDraft((d) => {
      const next = d.gender_preference.includes(g)
        ? d.gender_preference.filter((x) => x !== g)
        : [...d.gender_preference, g]
      return { ...d, gender_preference: next }
    })
  }

  const minAge = draft.min_age ?? AGE_MIN
  const maxAge = draft.max_age ?? AGE_MAX
  const maxDist = draft.max_distance_km ?? DIST_MAX

  return (
    <div className="space-y-6">
      {/* Age range */}
      <div className="space-y-3">
        <p className="text-xs font-semibold text-gray-400 uppercase tracking-wide">{t("ageRange")}</p>
        <div className="space-y-2">
          <div className="flex items-center justify-between text-xs text-gray-400 mb-1">
            <span>{t("ageMin")}</span>
            <span>{t("ageMax")}</span>
          </div>
          <RangeSlider
            min={AGE_MIN}
            max={maxAge}
            value={minAge}
            onChange={(v) => setDraft((d) => ({ ...d, min_age: v === AGE_MIN ? null : v }))}
          />
          <RangeSlider
            min={minAge}
            max={AGE_MAX}
            value={maxAge}
            onChange={(v) => setDraft((d) => ({ ...d, max_age: v === AGE_MAX ? null : v }))}
          />
        </div>
      </div>

      {/* Max distance */}
      <div className="space-y-3">
        <p className="text-xs font-semibold text-gray-400 uppercase tracking-wide">{t("maxDistance")}</p>
        <RangeSlider
          min={1}
          max={DIST_MAX}
          value={maxDist}
          onChange={(v) => setDraft((d) => ({ ...d, max_distance_km: v === DIST_MAX ? null : v }))}
          formatValue={(v) => v === DIST_MAX ? t("anyDistance") : `${v} km`}
        />
      </div>

      {/* Gender */}
      <div className="space-y-3">
        <p className="text-xs font-semibold text-gray-400 uppercase tracking-wide">{t("showMe")}</p>
        <div className="flex flex-wrap gap-2">
          {GENDER_OPTIONS.map((g) => {
            const active = draft.gender_preference.includes(g)
            return (
              <button
                key={g}
                type="button"
                onClick={() => toggleGender(g)}
                className={`rounded-full px-3 py-1 text-xs font-medium ring-1 transition-colors ${
                  active
                    ? "bg-brand-primary text-white ring-brand-hover"
                    : "bg-gray-800 text-gray-400 ring-gray-700 hover:text-gray-200"
                }`}
              >
                {g}
              </button>
            )
          })}
        </div>
      </div>

      {/* Interests */}
      <div className="space-y-3">
        <p className="text-xs font-semibold text-gray-400 uppercase tracking-wide">{t("filterInterests")}</p>
        <div className="relative">
          <input
            type="text"
            value={interestQuery}
            onChange={(e) => setInterestQuery(e.target.value)}
            placeholder={t("searchInterests")}
            className="w-full rounded bg-gray-800 px-3 py-1.5 text-sm text-white placeholder-gray-600 ring-1 ring-gray-700 focus:outline-none focus:ring-brand-hover"
          />
          {interestSuggestions.length > 0 && (
            <ul className="absolute z-10 mt-1 w-full rounded-md border border-gray-700 bg-gray-800 shadow-lg">
              {interestSuggestions.map((s) => (
                <li key={s}>
                  <button
                    type="button"
                    onClick={() => addInterest(s)}
                    className="w-full px-3 py-1.5 text-left text-sm text-gray-200 hover:bg-gray-700"
                  >
                    {s}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
        {draftInterests.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {draftInterests.map((tag) => (
              <span
                key={tag}
                className="flex items-center gap-1 rounded-full bg-brand-wash/50 px-2 py-0.5 text-xs text-brand-subtle ring-1 ring-brand-strong/60"
              >
                {tag}
                <button type="button" onClick={() => removeInterest(tag)} className="hover:text-white">×</button>
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Sort by distance */}
      <label className="flex items-center gap-2 cursor-pointer select-none">
        <input
          type="checkbox"
          checked={draftSort}
          onChange={(e) => setDraftSort(e.target.checked)}
          className="h-4 w-4 rounded border-gray-600 bg-gray-800 accent-brand-hover"
        />
        <span className="text-xs text-gray-300">{t("sortByDistance")}</span>
      </label>

      <Button
        variant="primary"
        size="sm"
        loading={saving}
        onClick={() => onApply(draft, draftSort, draftInterests)}
        className="w-full"
      >
        {t("apply")}
      </Button>
    </div>
  )
}

function activeFilterCount(prefs: Preferences, interests: string[]): number {
  return [
    prefs.min_age !== null || prefs.max_age !== null,
    prefs.max_distance_km !== null,
    prefs.gender_preference.length > 0,
    interests.length > 0,
  ].filter(Boolean).length
}

export default function BrowsePage() {
  const t = useTranslations("browse")
  const { data: session, status } = useSession()
  const token = session?.accessToken

  const [profiles, setProfiles] = useState<BrowseProfile[]>([])
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [initialLoading, setInitialLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState("")
  const [sheetOpen, setSheetOpen] = useState(false)

  const [prefs, setPrefs] = useState<Preferences>({
    min_age: null,
    max_age: null,
    max_distance_km: null,
    gender_preference: [],
  })
  const [savingPrefs, setSavingPrefs] = useState(false)
  const [sortByDistance, setSortByDistance] = useState(false)
  const [filterInterests, setFilterInterests] = useState<string[]>([])

  const fetchInFlight = useRef(false)
  const sentinelRef = useRef<HTMLDivElement>(null)

  const loadProfiles = useCallback(
    async (cursor: string | null, append: boolean, sortDist: boolean, interests: string[]) => {
      if (!token || fetchInFlight.current) return
      fetchInFlight.current = true
      if (!append) setInitialLoading(true)
      else setLoadingMore(true)
      setError("")

      try {
        const cursorParam = cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""
        const sortParam = sortDist ? "&sort=distance" : ""
        const interestParams = interests.map((i) => `&interests=${encodeURIComponent(i)}`).join("")
        const res = await fetch(
          `${API_URL}/profiles/browse?limit=${PAGE_SIZE}${cursorParam}${sortParam}${interestParams}`,
          { headers: { Authorization: `Bearer ${token}` } }
        )
        if (!res.ok) throw new Error("Failed to load profiles.")
        const data: BrowsePage = await res.json()
        setProfiles((prev) => (append ? [...prev, ...data.profiles] : data.profiles))
        setNextCursor(data.next_cursor || null)
      } catch (e: unknown) {
        setError(e instanceof Error ? e.message : "Something went wrong.")
      } finally {
        fetchInFlight.current = false
        setInitialLoading(false)
        setLoadingMore(false)
      }
    },
    [token]
  )

  useEffect(() => {
    if (status !== "authenticated" || !token) return

    fetch(`${API_URL}/profiles/me/preferences`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((r) => (r.ok ? r.json() : null))
      .then((data: Preferences | null) => { if (data) setPrefs(data) })
      .catch(() => {})

    loadProfiles(null, false, false, [])
  }, [status, token, loadProfiles])

  useEffect(() => {
    const el = sentinelRef.current
    if (!el) return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && nextCursor && !fetchInFlight.current) {
          loadProfiles(nextCursor, true, sortByDistance, filterInterests)
        }
      },
      { rootMargin: "200px" }
    )
    observer.observe(el)
    return () => observer.disconnect()
  }, [nextCursor, sortByDistance, filterInterests, loadProfiles])

  async function handleApplyFilters(updated: Preferences, newSort: boolean, newInterests: string[]) {
    if (!token) return
    setSavingPrefs(true)
    try {
      const res = await fetch(`${API_URL}/profiles/me/preferences`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify(updated),
      })
      if (res.ok) setPrefs(await res.json())
    } finally {
      setSavingPrefs(false)
    }
    setSortByDistance(newSort)
    setFilterInterests(newInterests)
    loadProfiles(null, false, newSort, newInterests)
  }

  async function handleClearFilters() {
    const empty: Preferences = { min_age: null, max_age: null, max_distance_km: null, gender_preference: [] }
    await handleApplyFilters(empty, false, [])
  }

  const filterCount = activeFilterCount(prefs, filterInterests)

  if (status === "loading" || initialLoading) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-10">
        <div className="mb-8">
          <Skeleton className="h-8 w-32" />
          <Skeleton className="mt-2 h-4 w-56" />
        </div>
        <div className="flex gap-6">
          <div className="hidden w-64 shrink-0 lg:block">
            <Skeleton className="h-96 w-full rounded-xl" />
          </div>
          <div className="grid flex-1 grid-cols-2 gap-4 sm:grid-cols-3">
            {Array.from({ length: PAGE_SIZE }).map((_, i) => (
              <ProfileCardSkeleton key={i} />
            ))}
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-6xl px-4 py-10">
      <div className="mb-8 flex items-end justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold text-white">{t("title")}</h1>
          <p className="mt-1 text-sm text-gray-400">{t("subtitle")}</p>
        </div>

        {/* Mobile: Filters button */}
        <button
          onClick={() => setSheetOpen(true)}
          className="lg:hidden flex items-center gap-2 rounded-full bg-gray-800 px-4 py-2 text-sm font-medium text-gray-200 ring-1 ring-gray-700 hover:bg-gray-700 transition-colors"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <line x1="4" y1="6" x2="20" y2="6" />
            <line x1="8" y1="12" x2="16" y2="12" />
            <line x1="11" y1="18" x2="13" y2="18" />
          </svg>
          {t("filters")}
          {filterCount > 0 && (
            <span className="flex h-4 min-w-4 items-center justify-center rounded-full bg-brand-primary px-1 text-[10px] font-semibold text-white">
              {filterCount}
            </span>
          )}
        </button>
      </div>

      {/* Mobile BottomSheet */}
      <BottomSheet open={sheetOpen} onClose={() => setSheetOpen(false)} title={t("filters")}>
        <FilterPanel
          prefs={prefs}
          sortByDistance={sortByDistance}
          selectedInterests={filterInterests}
          token={token!}
          saving={savingPrefs}
          onApply={(p, s, i) => { handleApplyFilters(p, s, i); setSheetOpen(false) }}
        />
      </BottomSheet>

      <div className="flex gap-6">
        {/* Desktop filter sidebar */}
        <aside className="hidden w-64 shrink-0 lg:block">
          <div className="sticky top-6 rounded-xl bg-gray-900 ring-1 ring-gray-800 p-5">
            <p className="mb-5 text-sm font-semibold text-white">{t("filters")}</p>
            <FilterPanel
              prefs={prefs}
              sortByDistance={sortByDistance}
              selectedInterests={filterInterests}
              token={token!}
              saving={savingPrefs}
              onApply={handleApplyFilters}
            />
          </div>
        </aside>

        {/* Results */}
        <div className="flex-1">
          {error && (
            <p className="mb-6 rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">
              {error}
            </p>
          )}

          {profiles.length === 0 && !error ? (
            <div className="flex flex-col items-center justify-center py-24 text-center gap-4">
              <p className="text-lg font-semibold text-gray-300">{t("noResults")}</p>
              {filterCount > 0 && (
                <Button variant="secondary" size="sm" onClick={handleClearFilters} loading={savingPrefs}>
                  {t("clearFilters")}
                </Button>
              )}
            </div>
          ) : (
            <>
              <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
                {profiles.map((profile) => (
                  <ProfileCard key={profile.id} profile={profile} token={token!} />
                ))}
                {loadingMore &&
                  Array.from({ length: PAGE_SIZE }).map((_, i) => (
                    <ProfileCardSkeleton key={`skel-${i}`} />
                  ))}
              </div>
              <div ref={sentinelRef} className="h-px" />
            </>
          )}
        </div>
      </div>
    </div>
  )
}
