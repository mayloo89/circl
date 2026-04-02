"use client"

import Image from "next/image"
import Link from "next/link"
import { useSession } from "next-auth/react"
import { useCallback, useEffect, useRef, useState } from "react"

import Avatar from "@/components/ui/Avatar"
import Button from "@/components/ui/Button"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
const PAGE_SIZE = 12

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
  has_more: boolean
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
    <div className="rounded-xl bg-gray-900 shadow-xl ring-1 ring-gray-800 overflow-hidden">
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
  const [status, setStatus] = useState<"idle" | "loading" | "sent" | "error">("idle")

  async function handleSend() {
    setStatus("loading")
    try {
      const res = await fetch(`${API_URL}/contacts`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ addressee_id: userID }),
      })
      if (res.ok || res.status === 409) {
        setStatus("sent")
      } else {
        setStatus("error")
      }
    } catch {
      setStatus("error")
    }
  }

  if (status === "sent") {
    return <span className="text-xs text-indigo-400 font-medium">Request sent</span>
  }

  return (
    <Button
      variant="primary"
      size="sm"
      onClick={(e) => { e.preventDefault(); handleSend() }}
      disabled={status === "loading"}
    >
      {status === "loading" ? "Sending…" : status === "error" ? "Retry" : "Add contact"}
    </Button>
  )
}

function ProfileCard({ profile, token }: { profile: BrowseProfile; token: string }) {
  const heroURL = profile.avatar_url
  const distance = formatDistance(profile.distance_km)

  return (
    <Link
      href={`/profile/${profile.username}`}
      className="group rounded-xl bg-gray-900 shadow-xl ring-1 ring-gray-800 overflow-hidden flex flex-col hover:ring-indigo-700 transition-shadow"
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
            {profile.location_text && (
              <>
                {profile.gender && <span>·</span>}
                <span>{profile.location_text}</span>
              </>
            )}
            {distance && (
              <>
                {(profile.gender || profile.location_text) && <span>·</span>}
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
                className="rounded-full bg-indigo-900/50 px-2 py-0.5 text-xs text-indigo-300 ring-1 ring-indigo-700/60"
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
  onApply: (p: Preferences, sortByDistance: boolean) => void
  saving: boolean
}

function FilterPanel({ prefs, sortByDistance, onApply, saving }: FilterPanelProps) {
  const [draft, setDraft] = useState<Preferences>(prefs)
  const [draftSort, setDraftSort] = useState(sortByDistance)

  // Sync when parent prefs load for the first time
  useEffect(() => { setDraft(prefs) }, [prefs])
  useEffect(() => { setDraftSort(sortByDistance) }, [sortByDistance])

  function toggleGender(g: string) {
    setDraft((d) => {
      const next = d.gender_preference.includes(g)
        ? d.gender_preference.filter((x) => x !== g)
        : [...d.gender_preference, g]
      return { ...d, gender_preference: next }
    })
  }

  function setInt(key: keyof Preferences, raw: string) {
    const n = parseInt(raw, 10)
    setDraft((d) => ({ ...d, [key]: isNaN(n) ? null : n }))
  }

  return (
    <div className="rounded-xl bg-gray-900 ring-1 ring-gray-800 p-5 space-y-5">
      <p className="text-sm font-semibold text-white">Filters</p>

      {/* Age range */}
      <div className="space-y-2">
        <p className="text-xs text-gray-400 font-medium">Age range</p>
        <div className="flex items-center gap-3">
          <input
            type="number"
            min={18}
            max={120}
            placeholder="Min"
            value={draft.min_age ?? ""}
            onChange={(e) => setInt("min_age", e.target.value)}
            className="w-20 rounded bg-gray-800 px-3 py-1.5 text-sm text-white placeholder-gray-600 ring-1 ring-gray-700 focus:outline-none focus:ring-indigo-500"
          />
          <span className="text-gray-500 text-sm">–</span>
          <input
            type="number"
            min={18}
            max={120}
            placeholder="Max"
            value={draft.max_age ?? ""}
            onChange={(e) => setInt("max_age", e.target.value)}
            className="w-20 rounded bg-gray-800 px-3 py-1.5 text-sm text-white placeholder-gray-600 ring-1 ring-gray-700 focus:outline-none focus:ring-indigo-500"
          />
        </div>
      </div>

      {/* Max distance */}
      <div className="space-y-2">
        <p className="text-xs text-gray-400 font-medium">Max distance (km)</p>
        <input
          type="number"
          min={1}
          placeholder="Any"
          value={draft.max_distance_km ?? ""}
          onChange={(e) => setInt("max_distance_km", e.target.value)}
          className="w-28 rounded bg-gray-800 px-3 py-1.5 text-sm text-white placeholder-gray-600 ring-1 ring-gray-700 focus:outline-none focus:ring-indigo-500"
        />
      </div>

      {/* Gender */}
      <div className="space-y-2">
        <p className="text-xs text-gray-400 font-medium">Show me</p>
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
                    ? "bg-indigo-600 text-white ring-indigo-500"
                    : "bg-gray-800 text-gray-400 ring-gray-700 hover:text-gray-200"
                }`}
              >
                {g}
              </button>
            )
          })}
        </div>
      </div>

      {/* Sort by distance */}
      <label className="flex items-center gap-2 cursor-pointer select-none">
        <input
          type="checkbox"
          checked={draftSort}
          onChange={(e) => setDraftSort(e.target.checked)}
          className="h-4 w-4 rounded border-gray-600 bg-gray-800 accent-indigo-500"
        />
        <span className="text-xs text-gray-300">Sort by distance</span>
      </label>

      <Button
        variant="primary"
        size="sm"
        loading={saving}
        onClick={() => onApply(draft, draftSort)}
        className="w-full"
      >
        Apply
      </Button>
    </div>
  )
}

export default function BrowsePage() {
  const { data: session, status } = useSession()
  const token = session?.accessToken

  const [profiles, setProfiles] = useState<BrowseProfile[]>([])
  const [hasMore, setHasMore] = useState(false)
  const [page, setPage] = useState(0)
  const [initialLoading, setInitialLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState("")

  const [prefs, setPrefs] = useState<Preferences>({
    min_age: null,
    max_age: null,
    max_distance_km: null,
    gender_preference: [],
  })
  const [savingPrefs, setSavingPrefs] = useState(false)
  const [sortByDistance, setSortByDistance] = useState(false)

  const fetchInFlight = useRef(false)

  const loadProfiles = useCallback(
    async (pageNum: number, append: boolean, sortDist: boolean) => {
      if (!token || fetchInFlight.current) return
      fetchInFlight.current = true
      if (pageNum === 0) setInitialLoading(true)
      else setLoadingMore(true)
      setError("")

      try {
        const sortParam = sortDist ? "&sort=distance" : ""
        const res = await fetch(
          `${API_URL}/profiles/browse?page=${pageNum}&limit=${PAGE_SIZE}${sortParam}`,
          { headers: { Authorization: `Bearer ${token}` } }
        )
        if (!res.ok) throw new Error("Failed to load profiles.")
        const data: BrowsePage = await res.json()
        setProfiles((prev) => (append ? [...prev, ...data.profiles] : data.profiles))
        setHasMore(data.has_more)
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

  // Load preferences + first page in parallel
  useEffect(() => {
    if (status !== "authenticated" || !token) return

    fetch(`${API_URL}/profiles/me/preferences`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((r) => (r.ok ? r.json() : null))
      .then((data: Preferences | null) => {
        if (data) setPrefs(data)
      })
      .catch(() => {})

    loadProfiles(0, false, false)
  }, [status, token, loadProfiles])

  async function handleApplyFilters(updated: Preferences, newSortByDistance: boolean) {
    if (!token) return
    setSavingPrefs(true)
    try {
      const res = await fetch(`${API_URL}/profiles/me/preferences`, {
        method: "PUT",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify(updated),
      })
      if (res.ok) {
        const saved: Preferences = await res.json()
        setPrefs(saved)
      }
    } finally {
      setSavingPrefs(false)
    }
    setSortByDistance(newSortByDistance)
    setPage(0)
    loadProfiles(0, false, newSortByDistance)
  }

  function handleLoadMore() {
    const next = page + 1
    setPage(next)
    loadProfiles(next, true, sortByDistance)
  }

  if (status === "loading" || initialLoading) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-10">
        <div className="mb-8">
          <Skeleton className="h-8 w-32" />
          <Skeleton className="mt-2 h-4 w-56" />
        </div>
        <div className="flex gap-6">
          <div className="hidden w-56 shrink-0 lg:block">
            <Skeleton className="h-64 w-full rounded-xl" />
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
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-white">Browse</h1>
        <p className="mt-1 text-sm text-gray-400">Discover people near you</p>
      </div>

      <div className="flex gap-6">
        {/* Filter sidebar */}
        <aside className="hidden w-56 shrink-0 lg:block">
          <FilterPanel prefs={prefs} sortByDistance={sortByDistance} onApply={handleApplyFilters} saving={savingPrefs} />
        </aside>

        {/* Results */}
        <div className="flex-1">
          {/* Mobile filter row */}
          <details className="mb-4 lg:hidden">
            <summary className="cursor-pointer text-sm text-indigo-400 hover:text-indigo-300 select-none">
              Filters
            </summary>
            <div className="mt-3">
              <FilterPanel prefs={prefs} sortByDistance={sortByDistance} onApply={handleApplyFilters} saving={savingPrefs} />
            </div>
          </details>

          {error && (
            <p className="mb-6 rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">
              {error}
            </p>
          )}

          {profiles.length === 0 && !error ? (
            <div className="flex flex-col items-center justify-center py-24 text-center">
              <p className="text-lg font-semibold text-gray-300">No profiles found</p>
              <p className="mt-2 text-sm text-gray-500">
                Try adjusting your filters or check back later.
              </p>
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

              {hasMore && !loadingMore && (
                <div className="mt-10 flex justify-center">
                  <Button variant="secondary" onClick={handleLoadMore}>
                    Load more
                  </Button>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}
