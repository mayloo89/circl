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
    return (
      <span className="text-xs text-indigo-400 font-medium">Request sent</span>
    )
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
  const heroURL = profile.first_photo_url || profile.avatar_url
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
            {distance && (
              <>
                {profile.gender && <span>·</span>}
                <span>{distance}</span>
              </>
            )}
            {!profile.gender && !distance && profile.location_text && (
              <span>{profile.location_text}</span>
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

export default function BrowsePage() {
  const { data: session, status } = useSession()
  const token = session?.accessToken

  const [profiles, setProfiles] = useState<BrowseProfile[]>([])
  const [hasMore, setHasMore] = useState(false)
  const [page, setPage] = useState(0)
  const [initialLoading, setInitialLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState("")

  const fetchInFlight = useRef(false)

  const loadProfiles = useCallback(
    async (pageNum: number, append: boolean) => {
      if (!token || fetchInFlight.current) return
      fetchInFlight.current = true
      if (pageNum === 1) setInitialLoading(true)
      else setLoadingMore(true)
      setError("")

      try {
        const res = await fetch(
          `${API_URL}/profiles/browse?page=${pageNum}&limit=${PAGE_SIZE}`,
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

  useEffect(() => {
    if (status !== "authenticated" || !token) return
    loadProfiles(0, false)
  }, [status, token, loadProfiles])

  function handleLoadMore() {
    const next = page + 1
    setPage(next)
    loadProfiles(next, true)
  }

  if (status === "loading" || initialLoading) {
    return (
      <div className="mx-auto max-w-5xl px-4 py-10">
        <div className="mb-8">
          <Skeleton className="h-8 w-32" />
          <Skeleton className="mt-2 h-4 w-56" />
        </div>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {Array.from({ length: PAGE_SIZE }).map((_, i) => (
            <ProfileCardSkeleton key={i} />
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-5xl px-4 py-10">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-white">Browse</h1>
        <p className="mt-1 text-sm text-gray-400">Discover people near you</p>
      </div>

      {error && (
        <p className="mb-6 rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">
          {error}
        </p>
      )}

      {profiles.length === 0 && !error ? (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <p className="text-lg font-semibold text-gray-300">No profiles found</p>
          <p className="mt-2 text-sm text-gray-500">
            Try updating your discovery preferences or check back later.
          </p>
        </div>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
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
  )
}
