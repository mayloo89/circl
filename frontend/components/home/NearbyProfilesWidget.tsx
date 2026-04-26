"use client"

import Image from "next/image"
import { useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { Link } from "@/i18n/navigation"
import Avatar from "@/components/ui/Avatar"
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
  const distanceLabel = formatDistance(profile.distance_km)
  const subtitle = distanceLabel ?? (profile.location_text || null)

  return (
    <Link
      href={`/profile/${profile.username}`}
      className="group flex-none w-28 flex flex-col items-center gap-1.5 rounded-card bg-gray-900 p-3 ring-1 ring-gray-800 hover:ring-brand-strong transition-all"
    >
      <div className="relative h-14 w-14 flex-none rounded-full overflow-hidden ring-1 ring-gray-700">
        {profile.avatar_url ? (
          <Image
            src={profile.avatar_url}
            alt=""
            fill
            className="object-cover group-hover:scale-105 transition-transform duration-300"
            sizes="56px"
          />
        ) : (
          <Avatar src="" name={profile.display_name || "?"} size="lg" className="!h-full !w-full !rounded-full" />
        )}
      </div>
      <p className="w-full text-center text-xs font-medium text-white truncate leading-tight">
        {profile.display_name}
      </p>
      {subtitle && (
        <p className="w-full text-center text-[11px] text-gray-500 truncate leading-tight">
          {subtitle}
        </p>
      )}
    </Link>
  )
}

function SkeletonChip() {
  return (
    <div className="flex-none w-28 flex flex-col items-center gap-1.5 rounded-card bg-gray-900 p-3 ring-1 ring-gray-800">
      <Skeleton className="h-14 w-14 rounded-full" />
      <Skeleton className="h-3 w-16" />
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
      <div className="mb-3 flex items-center justify-between">
        <h2 id="nearby-heading" className="text-sm font-semibold text-gray-400 uppercase tracking-wide">
          {t("nearbyPeople")}
        </h2>
        <Link href="/browse" className="text-xs text-brand-subtle hover:text-white transition-colors">
          {t("browseAll")} →
        </Link>
      </div>

      <div className="flex gap-2.5 overflow-x-auto py-1.5 scrollbar-hide snap-x snap-mandatory">
        {loading
          ? Array.from({ length: 6 }).map((_, i) => <SkeletonChip key={i} />)
          : profiles.length > 0
            ? profiles.map((p) => <ProfileChip key={p.user_id} profile={p} />)
            : (
              <p className="text-sm text-gray-500 py-2">{t("nearbyEmpty")}</p>
            )}
      </div>
    </section>
  )
}
