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

  // Clear the nav badge when the user is on the chat list page.
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

  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-950">
        <p className="text-gray-400">Loading...</p>
      </div>
    )
  }

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
          {rooms.length === 0 ? (
            <p className="p-6 text-sm text-gray-500">
              No conversations yet. Start one from a contact&apos;s profile.
            </p>
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
                          {room.last_message.content}
                        </p>
                      )}
                    </div>
                    {room.unread_count > 0 && (
                      <span className="ml-3 flex h-5 min-w-5 items-center justify-center rounded-full bg-indigo-600 px-1.5 text-[10px] font-semibold text-white">
                        {room.unread_count > 99 ? "99+" : room.unread_count}
                      </span>
                    )}
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
