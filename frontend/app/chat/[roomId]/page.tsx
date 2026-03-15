"use client"

import { useSession } from "next-auth/react"
import { useParams, useRouter } from "next/navigation"
import { useEffect, useRef, useState } from "react"

import { useChat, type ChatMessage } from "@/hooks/useChat"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface HistoryMessage {
  id: string
  room_id: string
  sender_id: string
  sender_name: string
  type: string
  content: string
  created_at: string
}

export default function ChatRoomPage() {
  const { data: session, status } = useSession()
  const router = useRouter()
  const params = useParams()
  const roomId = typeof params.roomId === "string" ? params.roomId : null

  const token = session?.accessToken
  const userID = session?.user?.id

  const [history, setHistory] = useState<HistoryMessage[]>([])
  const [input, setInput] = useState("")
  const bottomRef = useRef<HTMLDivElement>(null)

  const { messages: liveMessages, connected, send } = useChat(roomId, token)

  // Redirect unauthenticated users.
  useEffect(() => {
    if (status === "unauthenticated") router.push("/login")
  }, [status, router])

  // Load message history once authenticated.
  useEffect(() => {
    if (status !== "authenticated" || !token || !roomId) return
    fetch(`${API_URL}/chat/rooms/${roomId}/messages?limit=50`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((r) => r.json())
      .then((data: HistoryMessage[]) => {
        // API returns newest-first; reverse for chronological display.
        setHistory([...data].reverse())
      })
      .catch(() => {})
  }, [status, token, roomId])

  // Mark room as read when entering.
  useEffect(() => {
    if (status !== "authenticated" || !token || !roomId) return
    fetch(`${API_URL}/chat/rooms/${roomId}/read`, {
      method: "PUT",
      headers: { Authorization: `Bearer ${token}` },
    }).catch(() => {})
  }, [status, token, roomId])

  // Scroll to bottom when messages change.
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [history, liveMessages])

  function handleSend() {
    const content = input.trim()
    if (!content) return
    send(content)
    setInput("")
  }

  // Combine history with live WebSocket messages, deduplicating by id.
  const historyIds = new Set(history.map((m) => m.id))
  const dedupedLive = liveMessages.filter((m) => !historyIds.has(m.id))
  const allMessages: Array<HistoryMessage | ChatMessage> = [...history, ...dedupedLive]

  if (status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-950">
        <p className="text-gray-400">Loading...</p>
      </div>
    )
  }

  return (
    <div className="flex h-screen flex-col bg-gray-950">
      {/* Header */}
      <div className="flex items-center gap-4 border-b border-gray-800 bg-gray-900 px-4 py-3">
        <button
          onClick={() => router.push("/chat")}
          className="text-gray-400 hover:text-gray-200"
        >
          ←
        </button>
        <div className="flex items-center gap-2">
          <span
            className={`h-2 w-2 rounded-full ${connected ? "bg-green-400" : "bg-gray-600"}`}
          />
          <span className="text-sm text-gray-300">
            {connected ? "Connected" : "Connecting…"}
          </span>
        </div>
      </div>

      {/* Message list */}
      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-3">
        {allMessages.map((msg) => {
          const isOwn = msg.sender_id === userID
          return (
            <div
              key={msg.id}
              className={`flex flex-col ${isOwn ? "items-end" : "items-start"}`}
            >
              {!isOwn && (
                <span className="mb-1 text-xs text-gray-500">{msg.sender_name}</span>
              )}
              <div
                className={`max-w-xs rounded-2xl px-4 py-2 text-sm ${
                  isOwn
                    ? "rounded-br-sm bg-indigo-600 text-white"
                    : "rounded-bl-sm bg-gray-800 text-gray-100"
                }`}
              >
                {msg.content}
              </div>
              <span className="mt-1 text-[10px] text-gray-600">
                {new Date(msg.created_at).toLocaleTimeString([], {
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </span>
            </div>
          )
        })}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <div className="border-t border-gray-800 bg-gray-900 px-4 py-3">
        <div className="flex items-center gap-3">
          <input
            type="text"
            placeholder="Message…"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleSend()}
            className="flex-1 rounded-full border border-gray-700 bg-gray-800 px-4 py-2 text-sm text-white placeholder-gray-500 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          />
          <button
            onClick={handleSend}
            disabled={!connected || !input.trim()}
            className="rounded-full bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-40"
          >
            Send
          </button>
        </div>
      </div>
    </div>
  )
}
