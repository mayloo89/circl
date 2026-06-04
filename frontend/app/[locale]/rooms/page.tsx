"use client"

import { useRouter } from "@/i18n/navigation"
import { useCallback, useEffect, useState } from "react"
import { useTranslations } from "next-intl"

import Avatar from "@/components/ui/Avatar"
import Button from "@/components/ui/Button"
import Input from "@/components/ui/Input"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface PublicRoomSummary {
  id: string
  name: string
  description: string
  active_count: number
}

function RoomSkeleton() {
  return (
    <li className="flex items-center gap-4 px-5 py-4">
      <Skeleton className="h-10 w-10 flex-none rounded-full" />
      <div className="flex-1 space-y-2">
        <Skeleton className="h-3.5 w-28" />
        <Skeleton className="h-3 w-56 bg-gray-800/70" />
      </div>
      <Skeleton className="h-8 w-16 rounded" />
    </li>
  )
}

export default function PublicRoomsPage() {
  const t = useTranslations("guestRooms")
  const tc = useTranslations("common")
  const router = useRouter()
  const [rooms, setRooms] = useState<PublicRoomSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [query, setQuery] = useState("")

  const fetchRooms = useCallback(() => {
    fetch(`${API_URL}/guest/rooms`)
      .then((r) => (r.ok ? r.json() : Promise.reject(r.status)))
      .then((data: PublicRoomSummary[]) => { setRooms(Array.isArray(data) ? data : []); setError(""); setLoading(false) })
      .catch(() => { setError(t("failedLoad")); setLoading(false) })
  }, [t])

  const retry = useCallback(() => {
    setLoading(true)
    fetchRooms()
  }, [fetchRooms])

  useEffect(() => {
    const controller = new AbortController()
    fetch(`${API_URL}/guest/rooms`, { signal: controller.signal })
      .then((r) => (r.ok ? r.json() : Promise.reject(r.status)))
      .then((data: PublicRoomSummary[]) => { setRooms(Array.isArray(data) ? data : []); setLoading(false) })
      .catch(() => { if (!controller.signal.aborted) { setError(t("failedLoad")); setLoading(false) } })
    return () => controller.abort()
  }, [t])

  const filtered = rooms.filter(
    (r) => !query || r.name.toLowerCase().includes(query.toLowerCase()) || r.description.toLowerCase().includes(query.toLowerCase())
  )

  return (
    <div className="flex min-h-screen flex-col bg-gray-950">
      <div className="mx-auto w-full max-w-2xl space-y-6 px-4 py-6">
        <div>
          <h1 className="text-3xl font-bold text-foreground">{t("title")}</h1>
          <p className="mt-1 text-sm text-gray-500">{t("subtitle")}</p>
        </div>

        <Input
          labelHidden
          label={t("searchLabel")}
          placeholder={t("search")}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />

        {error && (
          <div className="flex items-center justify-between rounded-md bg-red-950 p-3 ring-1 ring-red-900">
            <p className="text-sm text-red-400">{error}</p>
            <Button variant="danger" size="sm" onClick={retry} className="ml-3 shrink-0">{tc("retry")}</Button>
          </div>
        )}

        <div className="rounded-lg bg-gray-900 shadow-xl ring-1 ring-gray-800">
          {loading ? (
            <ul className="divide-y divide-gray-800">
              {[0, 1, 2, 3].map((i) => <RoomSkeleton key={i} />)}
            </ul>
          ) : filtered.length === 0 ? (
            <div className="flex flex-col items-center gap-3 px-6 py-14 text-center">
              <div className="flex h-14 w-14 items-center justify-center rounded-full bg-gray-800">
                <svg className="h-7 w-7 text-gray-600" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5a17.92 17.92 0 01-8.716-2.247m0 0A8.966 8.966 0 013 12c0-1.264.26-2.466.732-3.558" />
                </svg>
              </div>
              <p className="text-sm font-medium text-gray-300">
                {query ? t("noResults") : t("noRooms")}
              </p>
            </div>
          ) : (
            <ul className="divide-y divide-gray-800">
              {filtered.map((room) => (
                <li key={room.id} className="flex items-center gap-4 px-5 py-4">
                  <Avatar name={room.name} size="md" color="indigo" />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold text-foreground">{room.name}</p>
                    {room.description && (
                      <p className="mt-0.5 truncate text-xs text-gray-400">{room.description}</p>
                    )}
                    {room.active_count > 0 && (
                      <p className="mt-0.5 text-xs text-gray-600">{t("onlineNow", { count: room.active_count })}</p>
                    )}
                  </div>
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={() => router.push(`/rooms/${room.id}`)}
                  >
                    {t("enter")}
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  )
}
