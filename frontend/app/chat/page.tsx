"use client"

import Image from "next/image"
import { useSession } from "next-auth/react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { useEffect, useState } from "react"

import { useNotificationsContext } from "@/contexts/NotificationsContext"

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

function lastMessagePreview(msg: MessageSummary): string {
  switch (msg.type) {
    case "image": return "📷 Photo"
    case "video": return "🎥 Video"
    case "file":  return "📎 File"
    default:      return msg.content || ""
  }
}

function RoomSkeleton() {
  return (
    <li className="flex items-center gap-3 px-6 py-4">
      <span className="h-10 w-10 flex-none animate-pulse rounded-full bg-gray-800" />
      <div className="flex-1 space-y-2">
        <span className="block h-3.5 w-32 animate-pulse rounded bg-gray-800" />
        <span className="block h-3 w-48 animate-pulse rounded bg-gray-800/70" />
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

  useEffect(() => {
    if (status !== "authenticated" || !token) return
    fetch(`${API_URL}/chat/rooms`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((r) => r.json())
      .then((data) => setRooms(data))
      .catch(() => setError("Failed to load conversations."))
      .finally(() => setLoading(false))
  }, [status, token])

  return (
    <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
      <div className="w-full max-w-lg space-y-6 px-4">
        <div className="flex items-center justify-between">
          <h1 className="text-3xl font-bold text-white">Messages</h1>
          <button
            onClick={() => router.push("/")}
            className="text-sm text-gray-400 hover:text-gray-200"
          >
            ← Home
          </button>
        </div>

        {error && (
          <p className="rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">{error}</p>
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
              <button
                onClick={() => router.push("/contacts")}
                className="mt-1 rounded-full bg-indigo-600 px-4 py-1.5 text-xs font-medium text-white hover:bg-indigo-500"
              >
                Go to contacts
              </button>
            </div>
          ) : (
            <ul className="divide-y divide-gray-800">
              {rooms.map((room) => (
                <li key={room.id}>
                  <Link
                    href={`/chat/${room.id}`}
                    className="flex items-center gap-3 px-6 py-4 hover:bg-gray-800/60 transition-colors"
                  >
                    {room.type === "dm" ? (
                      room.peer_avatar_url ? (
                        <Image src={room.peer_avatar_url} alt="" width={40} height={40} className="h-10 w-10 flex-none rounded-full object-cover ring-1 ring-gray-700" />
                      ) : (
                        <span className="flex h-10 w-10 flex-none items-center justify-center rounded-full bg-gray-700 text-sm font-medium text-gray-300 ring-1 ring-gray-600">
                          {(room.peer_name || "?")[0].toUpperCase()}
                        </span>
                      )
                    ) : (
                      <span className="flex h-10 w-10 flex-none items-center justify-center rounded-full bg-indigo-700 text-sm font-medium text-white ring-1 ring-indigo-600">
                        {(room.name || "G")[0].toUpperCase()}
                      </span>
                    )}
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-semibold text-white">
                        {room.type === "dm" ? room.peer_name || "Unknown" : room.name}
                      </p>
                      {room.last_message && (
                        <p className="mt-0.5 truncate text-xs text-gray-400">
                          {lastMessagePreview(room.last_message)}
                        </p>
                      )}
                    </div>
                    <div className="flex flex-none flex-col items-end gap-1.5">
                      {room.last_message && (
                        <span className="text-[10px] text-gray-600">
                          {relativeTime(room.last_message.created_at)}
                        </span>
                      )}
                      {room.unread_count > 0 && (
                        <span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-indigo-600 px-1.5 text-[10px] font-semibold text-white">
                          {room.unread_count > 99 ? "99+" : room.unread_count}
                        </span>
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
