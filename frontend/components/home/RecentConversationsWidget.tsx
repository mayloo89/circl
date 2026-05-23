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
    case "image": return <span className="flex items-center gap-1 text-gray-500"><svg className="h-3 w-3 flex-none" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true"><path strokeLinecap="round" strokeLinejoin="round" d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z" /></svg>Photo</span>
    case "video": return <span className="flex items-center gap-1 text-gray-500"><svg className="h-3 w-3 flex-none" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true"><path strokeLinecap="round" strokeLinejoin="round" d="m15.75 10.5 4.72-4.72a.75.75 0 0 1 1.28.53v11.38a.75.75 0 0 1-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 0 0 2.25-2.25v-9a2.25 2.25 0 0 0-2.25-2.25h-9A2.25 2.25 0 0 0 2.25 7.5v9a2.25 2.25 0 0 0 2.25 2.25Z" /></svg>Video</span>
    case "file": return <span className="flex items-center gap-1 text-gray-500"><svg className="h-3 w-3 flex-none" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true"><path strokeLinecap="round" strokeLinejoin="round" d="M18.375 12.739l-7.693 7.693a4.5 4.5 0 0 1-6.364-6.364l10.94-10.94A3 3 0 1 1 19.5 7.372L8.552 18.32m.009-.01-.01.01m5.699-9.941-7.81 7.81a1.5 1.5 0 0 0 2.112 2.13" /></svg>File</span>
    case "album_share": return <span className="flex items-center gap-1 text-gray-500"><svg className="h-3 w-3 flex-none" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="3" width="18" height="18" rx="2" /><circle cx="9" cy="9" r="2" /><path d="M21 15l-5-5L5 21" /></svg>Album</span>
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
          <span className="text-sm font-medium text-foreground truncate">{room.name}</span>
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
        <span className="flex-none h-4 min-w-4 rounded-full bg-brand-primary px-1 text-[10px] font-semibold text-foreground flex items-center justify-center">
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
        <Link href="/chat" className="text-xs text-brand-subtle hover:text-foreground transition-colors">
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
