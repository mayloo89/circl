"use client"

import { useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { Link } from "@/i18n/navigation"
import Avatar from "@/components/ui/Avatar"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface RoomSummary {
  id: string
  type: "dm" | "group" | "channel"
  name: string
  peer_avatar_url: string
  last_message: { content: string; type?: string; created_at: string } | null
  unread_count: number
}

function relativeTime(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60_000)
  if (mins < 1) return "now"
  if (mins < 60) return `${mins}m`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  if (days === 1) return "yesterday"
  if (days < 7) return new Date(dateStr).toLocaleDateString(undefined, { weekday: "short" })
  return new Date(dateStr).toLocaleDateString(undefined, { month: "short", day: "numeric" })
}

function MessagePreview({ msg }: { msg: RoomSummary["last_message"] }) {
  if (!msg) return null
  switch (msg.type) {
    case "image": return <span className="text-gray-500">📷 Photo</span>
    case "video": return <span className="text-gray-500">🎥 Video</span>
    case "file": return <span className="text-gray-500">📎 File</span>
    default: return <span className="truncate">{msg.content}</span>
  }
}

function RoomRow({ room }: { room: RoomSummary }) {
  const isGroup = room.type !== "dm"
  return (
    <Link
      href={`/chat/${room.id}`}
      className="flex items-center gap-3 rounded-card bg-gray-900 shadow-card ring-1 ring-gray-800 p-3 hover:ring-brand-strong transition-all"
    >
      <Avatar
        src={room.peer_avatar_url}
        name={room.name || "?"}
        size="md"
        color={isGroup ? "indigo" : "gray"}
      />
      <div className="flex-1 min-w-0">
        <div className="flex items-baseline justify-between gap-2">
          <span className="text-sm font-medium text-white truncate">{room.name}</span>
          {room.last_message && (
            <span className="flex-none text-xs text-gray-500">
              {relativeTime(room.last_message.created_at)}
            </span>
          )}
        </div>
        <p className="mt-0.5 text-xs text-gray-400 truncate">
          <MessagePreview msg={room.last_message} />
        </p>
      </div>
      {room.unread_count > 0 && (
        <span className="flex-none h-4 min-w-4 rounded-full bg-brand-primary px-1 text-[10px] font-semibold text-white flex items-center justify-center">
          {room.unread_count > 9 ? "9+" : room.unread_count}
        </span>
      )}
    </Link>
  )
}

function SkeletonRow() {
  return (
    <div className="flex items-center gap-3 rounded-card bg-gray-900 p-3 ring-1 ring-gray-800">
      <Skeleton className="h-8 w-8 rounded-full flex-none" />
      <div className="flex-1 space-y-1.5">
        <Skeleton className="h-3 w-32" />
        <Skeleton className="h-2.5 w-48" />
      </div>
    </div>
  )
}

export default function RecentConversationsWidget() {
  const t = useTranslations("home")
  const { data: session, status } = useSession()
  const [rooms, setRooms] = useState<RoomSummary[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (status !== "authenticated" || !session?.accessToken) return
    const token = session.accessToken
    let cancelled = false
    fetch(`${API_URL}/chat/rooms?limit=5`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => (res.ok ? res.json() : Promise.reject()))
      .then((data) => {
        if (!cancelled) setRooms(Array.isArray(data) ? data : (data.rooms ?? []))
      })
      .catch(() => {})
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [status, session?.accessToken])

  return (
    <section aria-labelledby="recent-heading">
      <div className="mb-3 flex items-center justify-between">
        <h2 id="recent-heading" className="text-sm font-semibold text-gray-400 uppercase tracking-wide">
          {t("recentConversations")}
        </h2>
        <Link href="/chat" className="text-xs text-brand-subtle hover:text-white transition-colors">
          {t("seeAllChats")} →
        </Link>
      </div>

      <div className="space-y-2">
        {loading ? (
          Array.from({ length: 3 }).map((_, i) => <SkeletonRow key={i} />)
        ) : rooms.length > 0 ? (
          rooms.map((room) => <RoomRow key={room.id} room={room} />)
        ) : (
          <p className="text-sm text-gray-500 py-2">{t("recentEmpty")}</p>
        )}
      </div>
    </section>
  )
}
