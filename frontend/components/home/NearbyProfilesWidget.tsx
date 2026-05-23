"use client"

import Image from "next/image"
import { useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { Link } from "@/i18n/navigation"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface NearbyProfile {
  user_id: string
  username: string
  display_name: string
  avatar_url: string
  distance_km: number | null
  location_text: string
}

function formatDistance(km: number | null): string | null {
  if (km === null) return null
  if (km < 1) return "< 1 km"
  return `${Math.round(km)} km`
}

function ProfileChip({ profile }: { profile: NearbyProfile }) {
  const [imgError, setImgError] = useState(false)
  const distanceLabel = formatDistance(profile.distance_km)
  const subtitle = distanceLabel ?? (profile.location_text || null)
  const initial = (profile.display_name || "?")[0].toUpperCase()
  const showImage = !!profile.avatar_url && !imgError

  return (
    <Link
      href={`/profile/${profile.username}`}
      className="group flex-none w-20 flex flex-col items-center gap-1.5 rounded-xl p-2 hover:bg-white/[0.06] transition-colors snap-start"
    >
      <div className="relative h-12 w-12 flex-none">
        {showImage ? (
          <Image
            src={profile.avatar_url}
            alt={profile.display_name ?? "Nearby user"}
            width={48}
            height={48}
            onError={() => setImgError(true)}
            className="h-12 w-12 rounded-full object-cover ring-2 ring-brand-primary dark:ring-white/20"
            sizes="48px"
          />
        ) : (
          <span className="flex h-12 w-12 items-center justify-center rounded-full bg-brand-primary/50 dark:bg-gray-700 text-sm font-semibold text-brand-primary dark:text-gray-300 ring-2 ring-brand-primary dark:ring-white/20">
            {initial}
          </span>
        )}
      </div>
      <p className="w-full text-center text-xs font-medium text-foreground truncate leading-tight">
        {profile.display_name}
      </p>
      {subtitle && (
        <p className="w-full text-center text-[10px] text-gray-500 truncate leading-tight">
          {subtitle}
        </p>
      )}
    </Link>
  )
}

function SkeletonChip() {
  return (
    <div className="flex-none w-20 flex flex-col items-center gap-1.5 p-2">
      <Skeleton className="h-12 w-12 rounded-full" />
      <Skeleton className="h-2.5 w-12" />
    </div>
  )
}

export default function NearbyProfilesWidget() {
  const t = useTranslations("home")
  const { data: session, status } = useSession()
  const [profiles, setProfiles] = useState<NearbyProfile[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (status !== "authenticated" || !session?.accessToken) return
    const token = session.accessToken
    let cancelled = false
    fetch(`${API_URL}/profiles/browse?limit=8`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => (res.ok ? res.json() : Promise.reject()))
      .then((data) => {
        if (!cancelled) setProfiles(data.profiles ?? [])
      })
      .catch(() => {})
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [status, session?.accessToken])

  return (
    <section aria-labelledby="nearby-heading">
      <div className="rounded-card bg-gray-900 dark:bg-white/[0.04] shadow-card ring-1 ring-brand-primary/20 dark:ring-white/[0.08] p-4">
        <div className="mb-3 flex items-center justify-between">
          <h2 id="nearby-heading" className="flex items-center gap-2 text-sm font-semibold text-foreground">
            <svg aria-hidden="true" className="h-4 w-4 flex-none text-brand-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
              <path strokeLinecap="round" strokeLinejoin="round" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
              <path strokeLinecap="round" strokeLinejoin="round" d="M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
            {t("nearbyPeople")}
          </h2>
          <Link href="/browse" className="text-xs text-brand-subtle hover:text-brand-primary transition-colors">
            {t("browseAll")} →
          </Link>
        </div>

        <div className="flex gap-1 overflow-x-auto py-0.5 scrollbar-hide snap-x snap-mandatory -mx-1 px-1">
          {loading
            ? Array.from({ length: 6 }).map((_, i) => <SkeletonChip key={i} />)
            : profiles.length > 0
              ? profiles.map((p) => <ProfileChip key={p.user_id} profile={p} />)
              : (
                <div className="flex flex-col gap-1.5 py-2">
                  <p className="text-sm text-gray-500">{t("nearbyEmpty")}</p>
                  <Link href="/profile" className="text-xs text-brand-subtle hover:text-brand-primary transition-colors">
                    {t("nearbySetLocation")} →
                  </Link>
                </div>
              )}
        </div>
      </div>
    </section>
  )
}
