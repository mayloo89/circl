"use client"

import Link from "next/link"
import { useSession } from "next-auth/react"
import { useRouter } from "next/navigation"
import { useEffect, useState } from "react"

import { useNotificationsContext } from "@/contexts/NotificationsContext"
import Avatar from "@/components/ui/Avatar"
import Badge from "@/components/ui/Badge"
import Button from "@/components/ui/Button"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface MessageSummary {
  sender_id: string
  content: string
  type?: string
  created_at: string
}

interface RoomSummary {
  id: string
  type: "dm" | "group"
  name: string
  peer_id: string
  peer_name: string
  peer_avatar_url: string
  last_message: MessageSummary | null
  unread_count: number
  created_at: string
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
  if (days < 7) return new Date(dateStr).toLocaleDateString([], { weekday: "short" })
  return new Date(dateStr).toLocaleDateString([], { month: "short", day: "numeric" })
}

function LastMessagePreview({ msg }: { msg: MessageSummary }) {
  switch (msg.type) {
    case "image":
      return (
        <span className="flex items-center gap-1">
          <svg className="h-3 w-3 flex-none" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z" />
          </svg>
          Photo
        </span>
      )
    case "video":
      return (
        <span className="flex items-center gap-1">
          <svg className="h-3 w-3 flex-none" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" d="m15.75 10.5 4.72-4.72a.75.75 0 0 1 1.28.53v11.38a.75.75 0 0 1-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 0 0 2.25-2.25v-9a2.25 2.25 0 0 0-2.25-2.25h-9A2.25 2.25 0 0 0 2.25 7.5v9a2.25 2.25 0 0 0 2.25 2.25Z" />
          </svg>
          Video
        </span>
      )
    case "file":
      return (
        <span className="flex items-center gap-1">
          <svg className="h-3 w-3 flex-none" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" d="M18.375 12.739l-7.693 7.693a4.5 4.5 0 0 1-6.364-6.364l10.94-10.94A3 3 0 1 1 19.5 7.372L8.552 18.32m.009-.01-.01.01m5.699-9.941-7.81 7.81a1.5 1.5 0 0 0 2.112 2.13" />
          </svg>
          File
        </span>
      )
    default:
      return <>{msg.content || ""}</>
  }
}

function RoomSkeleton() {
  return (
    <li className="flex items-center gap-3 px-6 py-4">
      <Skeleton className="h-10 w-10 flex-none rounded-full" />
      <div className="flex-1 space-y-2">
        <Skeleton className="h-3.5 w-32" />
        <Skeleton className="h-3 w-48 bg-gray-800/70" />
      </div>
    </li>
  )
}

export default function ChatPage() {
  const { data: session, status } = useSession()
  const router = useRouter()
  const [rooms, setRooms] = useState<RoomSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  const token = session?.accessToken
  const { clearChatBadge } = useNotificationsContext()

  useEffect(() => {
    if (status === "unauthenticated") router.push("/login")
  }, [status, router])

  useEffect(() => { clearChatBadge() }, [clearChatBadge])

  function loadRooms() {
    if (!token) return
    setError("")
    setLoading(true)
    fetch(`${API_URL}/chat/rooms`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((r) => r.json())
      .then((data) => setRooms(data))
      .catch(() => setError("Failed to load conversations."))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    if (status !== "authenticated" || !token) return
    loadRooms()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, token])

  return (
    <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
      <div className="w-full max-w-lg space-y-6 px-4">
        <div className="flex items-center justify-between">
          <h1 className="text-3xl font-bold text-white">Messages</h1>
          <Button variant="ghost" aria-label="Go to home" onClick={() => router.push("/")}>← Home</Button>
        </div>

        {error && (
          <div className="flex items-center justify-between rounded-md bg-red-950 p-3 ring-1 ring-red-900">
            <p className="text-sm text-red-400">{error}</p>
            <Button variant="danger" size="sm" onClick={loadRooms} className="ml-3 shrink-0">Retry</Button>
          </div>
        )}

        <div className="rounded-lg bg-gray-900 shadow-xl ring-1 ring-gray-800">
          {status === "loading" || loading ? (
            <ul className="divide-y divide-gray-800">
              {[0, 1, 2].map((i) => <RoomSkeleton key={i} />)}
            </ul>
          ) : rooms.length === 0 ? (
            <div className="flex flex-col items-center gap-4 px-6 py-14 text-center">
              <div className="flex h-16 w-16 items-center justify-center rounded-full bg-gray-800">
                <svg className="h-8 w-8 text-gray-600" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M20.25 8.511c.884.284 1.5 1.128 1.5 2.097v4.286c0 1.136-.847 2.1-1.98 2.193-.34.027-.68.052-1.02.072v3.091l-3-3c-1.354 0-2.694-.055-4.02-.163a2.115 2.115 0 01-.825-.242m9.345-8.334a2.126 2.126 0 00-.476-.095 48.64 48.64 0 00-8.048 0c-1.131.094-1.976 1.057-1.976 2.192v4.286c0 .837.46 1.58 1.155 1.951m9.345-8.334V6.637c0-1.621-1.152-3.026-2.76-3.235A48.455 48.455 0 0011.25 3c-2.115 0-4.198.137-6.24.402-1.608.209-2.76 1.614-2.76 3.235v6.226c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.14 1.74.194V21l4.155-4.155" />
                </svg>
              </div>
              <div>
                <p className="text-sm font-medium text-gray-300">No conversations yet</p>
                <p className="mt-1 text-xs text-gray-500">Start a chat from a contact&apos;s profile.</p>
              </div>
              <Button variant="primary" size="sm" pill onClick={() => router.push("/contacts")} className="mt-1">
                Go to contacts
              </Button>
            </div>
          ) : (
            <ul className="divide-y divide-gray-800">
              {rooms.map((room) => (
                <li key={room.id}>
                  <Link
                    href={`/chat/${room.id}`}
                    className="flex items-center gap-3 px-6 py-4 transition-colors hover:bg-gray-800/60"
                  >
                    <Avatar
                      src={room.type === "dm" ? room.peer_avatar_url : undefined}
                      name={room.type === "dm" ? (room.peer_name || "?") : (room.name || "G")}
                      size="lg"
                      color={room.type === "group" ? "indigo" : "gray"}
                    />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-semibold text-white">
                        {room.type === "dm" ? room.peer_name || "Unknown" : room.name}
                      </p>
                      {room.last_message && (
                        <div className="mt-0.5 truncate text-xs text-gray-400">
                          <LastMessagePreview msg={room.last_message} />
                        </div>
                      )}
                    </div>
                    <div className="flex flex-none flex-col items-end gap-1.5">
                      {room.last_message && (
                        <span className="text-xs text-gray-600">
                          {relativeTime(room.last_message.created_at)}
                        </span>
                      )}
                      {room.unread_count > 0 && (
                        <Badge count={room.unread_count} max={99} />
                      )}
                    </div>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  )
}
