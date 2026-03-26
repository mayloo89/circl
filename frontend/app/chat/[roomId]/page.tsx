"use client"

import Image from "next/image"
import { useSession } from "next-auth/react"
import { useParams, useRouter } from "next/navigation"
import { useEffect, useRef, useState } from "react"

import { useNotificationsContext } from "@/contexts/NotificationsContext"
import { useChat, type ChatMessage, type SendOpts } from "@/hooks/useChat"
import { usePresence, formatLastSeen } from "@/hooks/usePresence"
import { useUpload } from "@/hooks/useUpload"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

type EphemeralMode = "off" | "view_once" | "15m" | "30m" | "1h" | "6h" | "12h" | "24h"

const EPHEMERAL_LABELS: Record<EphemeralMode, string> = {
  off: "Off",
  view_once: "View once",
  "15m": "15 minutes",
  "30m": "30 minutes",
  "1h": "1 hour",
  "6h": "6 hours",
  "12h": "12 hours",
  "24h": "24 hours",
}

interface HistoryMessage {
  id: string
  room_id: string
  sender_id: string
  sender_name: string
  sender_avatar_url: string
  type: string
  content: string
  thumbnail_url?: string
  view_once: boolean
  tombstone?: boolean
  expires_at?: string
  created_at: string
}

interface RoomSummary {
  id: string
  type: "dm" | "group"
  name: string
  peer_id: string
  peer_name: string
  peer_avatar_url: string
  peer_last_read_at?: string
}

type AnyMessage = HistoryMessage | ChatMessage

// ─── Helpers ─────────────────────────────────────────────────────────────────

function sameCalendarDay(a: string, b: string): boolean {
  const da = new Date(a), db = new Date(b)
  return (
    da.getFullYear() === db.getFullYear() &&
    da.getMonth() === db.getMonth() &&
    da.getDate() === db.getDate()
  )
}

function formatDaySeparator(dateStr: string): string {
  const now = new Date()
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  if (sameCalendarDay(dateStr, now.toISOString())) return "Today"
  if (sameCalendarDay(dateStr, yesterday.toISOString())) return "Yesterday"
  return new Date(dateStr).toLocaleDateString([], { weekday: "long", month: "long", day: "numeric" })
}

const GROUP_BREAK_MS = 5 * 60_000 // 5 minutes

function isFirstInGroup(msgs: AnyMessage[], i: number): boolean {
  if (i === 0) return true
  const prev = msgs[i - 1]
  const cur = msgs[i]
  if (prev.sender_id !== cur.sender_id) return true
  if (!sameCalendarDay(prev.created_at, cur.created_at)) return true
  if (new Date(cur.created_at).getTime() - new Date(prev.created_at).getTime() > GROUP_BREAK_MS) return true
  if (prev.tombstone) return true
  return false
}

function isLastInGroup(msgs: AnyMessage[], i: number): boolean {
  if (i === msgs.length - 1) return true
  const cur = msgs[i]
  const next = msgs[i + 1]
  if (cur.sender_id !== next.sender_id) return true
  if (!sameCalendarDay(cur.created_at, next.created_at)) return true
  if (new Date(next.created_at).getTime() - new Date(cur.created_at).getTime() > GROUP_BREAK_MS) return true
  if (cur.tombstone) return true
  return false
}

// ─── Lightbox ────────────────────────────────────────────────────────────────

function Lightbox({ url, type, onClose }: { url: string; type: string; onClose: () => void }) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) { if (e.key === "Escape") onClose() }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [onClose])

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/90"
      onClick={onClose}
    >
      <button
        aria-label="Close"
        className="absolute right-4 top-4 rounded-full p-2 text-white/70 hover:text-white"
        onClick={onClose}
      >
        <svg className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
      {type === "video" ? (
        <video
          src={url}
          controls
          autoPlay
          className="max-h-[90vh] max-w-[90vw] rounded-lg"
          onClick={(e) => e.stopPropagation()}
        />
      ) : (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={url}
          alt=""
          className="max-h-[90vh] max-w-[90vw] rounded-lg object-contain"
          onClick={(e) => e.stopPropagation()}
        />
      )}
    </div>
  )
}

// ─── Skeletons ────────────────────────────────────────────────────────────────

function MessageSkeletons() {
  return (
    <div className="space-y-4 px-4 py-4">
      {/* incoming */}
      <div className="flex items-end gap-2">
        <span className="h-7 w-7 flex-none animate-pulse rounded-full bg-gray-800" />
        <div className="space-y-1">
          <span className="block h-3 w-16 animate-pulse rounded bg-gray-800/60" />
          <span className="block h-9 w-48 animate-pulse rounded-2xl bg-gray-800" />
        </div>
      </div>
      {/* outgoing */}
      <div className="flex justify-end">
        <span className="block h-9 w-36 animate-pulse rounded-2xl bg-indigo-900/50" />
      </div>
      {/* incoming long */}
      <div className="flex items-end gap-2">
        <span className="h-7 w-7 flex-none animate-pulse rounded-full bg-gray-800" />
        <span className="block h-9 w-64 animate-pulse rounded-2xl bg-gray-800" />
      </div>
      {/* outgoing */}
      <div className="flex justify-end">
        <span className="block h-9 w-52 animate-pulse rounded-2xl bg-indigo-900/50" />
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function ChatRoomPage() {
  const { data: session, status } = useSession()
  const router = useRouter()
  const params = useParams()
  const roomId = typeof params.roomId === "string" ? params.roomId : null

  const token = session?.accessToken
  const userID = session?.user?.id

  const [history, setHistory] = useState<HistoryMessage[]>([])
  const [historyLoading, setHistoryLoading] = useState(true)
  const [input, setInput] = useState("")
  const [room, setRoom] = useState<RoomSummary | null>(null)
  const [ephemeral, setEphemeral] = useState<EphemeralMode>("off")
  const [showEphemeralMenu, setShowEphemeralMenu] = useState(false)
  const [mediaModal, setMediaModal] = useState<{ url: string; type: string } | null>(null)
  const [revealedMessages, setRevealedMessages] = useState<Map<string, { msg: HistoryMessage | ChatMessage; content: string }>>(new Map())
  const bottomRef = useRef<HTMLDivElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const typingThrottleRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const { messages: liveMessages, deletedIds, connected, send, sendAttachment, sendTyping, typingUsers, readReceipts } = useChat(roomId, token)
  const { upload, uploading } = useUpload(token)
  const { clearChatBadge, subscribe } = useNotificationsContext()

  const peerIDs = room?.peer_id ? [room.peer_id] : []
  const presence = usePresence(peerIDs, token, subscribe)

  useEffect(() => { clearChatBadge() }, [clearChatBadge])

  useEffect(() => {
    if (status === "unauthenticated") router.push("/login")
  }, [status, router])

  useEffect(() => {
    if (status !== "authenticated" || !token || !roomId) return
    fetch(`${API_URL}/chat/rooms`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => r.json())
      .then((rooms: RoomSummary[]) => {
        const found = rooms.find((r) => r.id === roomId)
        if (found) setRoom(found)
      })
      .catch(() => {})
  }, [status, token, roomId])

  useEffect(() => {
    if (status !== "authenticated" || !token || !roomId) return
    fetch(`${API_URL}/chat/rooms/${roomId}/messages?limit=50`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((r) => r.json())
      .then((data: HistoryMessage[]) => setHistory([...data].reverse()))
      .catch(() => {})
      .finally(() => setHistoryLoading(false))
  }, [status, token, roomId])

  useEffect(() => {
    if (status !== "authenticated" || !token || !roomId) return
    fetch(`${API_URL}/chat/rooms/${roomId}/read`, {
      method: "PUT",
      headers: { Authorization: `Bearer ${token}` },
    }).catch(() => {})
  }, [status, token, roomId])

  useEffect(() => {
    if (!token || !roomId || liveMessages.length === 0) return

    function markRead() {
      if (document.visibilityState !== "visible") return
      const last = liveMessages[liveMessages.length - 1]
      if (last?.sender_id !== userID) {
        fetch(`${API_URL}/chat/rooms/${roomId}/read`, {
          method: "PUT",
          headers: { Authorization: `Bearer ${token}` },
        }).catch(() => {})
      }
    }

    markRead()
    document.addEventListener("visibilitychange", markRead)
    return () => document.removeEventListener("visibilitychange", markRead)
  }, [liveMessages, roomId, token, userID])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [history, liveMessages])

  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 60_000)
    return () => clearInterval(id)
  }, [])

  function buildOpts(): SendOpts | undefined {
    if (ephemeral === "off") return undefined
    if (ephemeral === "view_once") return { viewOnce: true }
    return { ttl: ephemeral }
  }

  function handleSend() {
    const content = input.trim()
    if (!content) return
    send(content, buildOpts())
    setInput("")
  }

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    e.target.value = ""
    const result = await upload(file, "chat-attachment")
    if (result) {
      sendAttachment(result.upload_id, result.url, file.type, buildOpts())
    }
  }

  // Combine history with live messages, deduplicating by id.
  const seen = new Set<string>()
  const allMessages: AnyMessage[] = []
  for (const msg of [...history, ...liveMessages]) {
    if (!seen.has(msg.id)) {
      seen.add(msg.id)
      allMessages.push(msg)
    }
  }
  for (const [id, { msg }] of revealedMessages) {
    if (!seen.has(id)) {
      seen.add(id)
      allMessages.push(msg)
    }
  }

  // IDs that came from the initial history load — used to skip animation for them.
  const historyIdSet = new Set(history.map((m) => m.id))

  async function handleViewOnce(msgId: string) {
    if (!token || !roomId) return
    const msgObj = allMessages.find((m) => m.id === msgId)
    const res = await fetch(`${API_URL}/chat/rooms/${roomId}/messages/${msgId}/view`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!res.ok || !msgObj) return
    if (msgObj.type === "image" || msgObj.type === "video") {
      const blob = await res.blob()
      setMediaModal({ url: URL.createObjectURL(blob), type: msgObj.type })
    } else {
      const data = await res.json()
      setRevealedMessages((prev) => {
        const next = new Map(prev)
        next.set(msgId, { msg: msgObj, content: data.content as string })
        return next
      })
    }
  }

  function formatExpiry(expiresAt: string, nowMs: number): string {
    const diff = new Date(expiresAt).getTime() - nowMs
    if (diff <= 0) return "expired"
    const h = Math.floor(diff / 3_600_000)
    const m = Math.floor((diff % 3_600_000) / 60_000)
    if (h >= 24) return `${Math.floor(h / 24)}d left`
    if (h >= 1) return `${h}h left`
    if (m >= 1) return `${m}m left`
    return "< 1m"
  }

  function expiryColorClass(expiresAt: string, nowMs: number): string {
    const diff = new Date(expiresAt).getTime() - nowMs
    const mins = diff / 60_000
    if (mins < 10) return "text-red-400"
    if (mins < 60) return "text-amber-400"
    return "text-gray-500"
  }

  const expiredIds = new Set(
    allMessages
      .filter((m) => m.expires_at && new Date(m.expires_at).getTime() <= now)
      .map((m) => m.id),
  )

  const peerId = room?.peer_id ?? ""
  const peerReadAtMs = Math.max(
    room?.peer_last_read_at ? new Date(room.peer_last_read_at).getTime() : 0,
    readReceipts.get(peerId) ?? 0,
  )

  // ID of the last own message the peer has seen — shows the "Seen" label.
  const lastSeenOwnMsgId = peerReadAtMs > 0
    ? [...allMessages].reverse().find(
        (m) => m.sender_id === userID && new Date(m.created_at).getTime() <= peerReadAtMs
      )?.id
    : undefined

  if (status === "loading") {
    return (
      <div className="flex h-full items-center justify-center bg-gray-950">
        <p className="text-gray-400">Loading...</p>
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col bg-gray-950">
      {mediaModal && (
        <Lightbox
          url={mediaModal.url}
          type={mediaModal.type}
          onClose={() => {
            if (mediaModal.url.startsWith("blob:")) URL.revokeObjectURL(mediaModal.url)
            setMediaModal(null)
          }}
        />
      )}

      {/* Header */}
      <div className="flex items-center gap-4 border-b border-gray-800 bg-gray-900 px-4 py-3">
        <button aria-label="Back to messages" onClick={() => router.push("/chat")} className="text-gray-400 hover:text-gray-200">
          ←
        </button>
        {room ? (
          <>
            {room.type === "dm" ? (
              room.peer_avatar_url ? (
                <Image src={room.peer_avatar_url} alt="" width={32} height={32} className="h-8 w-8 flex-none rounded-full object-cover ring-1 ring-gray-700" />
              ) : (
                <span className="flex h-8 w-8 flex-none items-center justify-center rounded-full bg-gray-700 text-sm text-gray-300 ring-1 ring-gray-600">
                  {(room.peer_name || "?")[0].toUpperCase()}
                </span>
              )
            ) : null}
            <div className="flex flex-col">
              <span className="text-sm font-medium text-white">
                {room.type === "dm" ? room.peer_name : room.name}
              </span>
              {room.type === "dm" && room.peer_id ? (
                <div className="flex items-center gap-1.5">
                  <span className={`h-1.5 w-1.5 rounded-full ${presence[room.peer_id]?.online ? "bg-green-400" : "bg-gray-600"}`} />
                  <span className="text-xs text-gray-400">
                    {presence[room.peer_id]?.online
                      ? "Online"
                      : presence[room.peer_id]?.last_seen_at
                      ? formatLastSeen(presence[room.peer_id].last_seen_at)
                      : "Offline"}
                  </span>
                </div>
              ) : null}
            </div>
          </>
        ) : (
          <div className="flex items-center gap-2">
            <span className={`h-2 w-2 rounded-full ${connected ? "bg-green-400" : "bg-gray-600"}`} />
            <span className="text-sm text-gray-300">{connected ? "Connected" : "Connecting…"}</span>
          </div>
        )}
      </div>

      {/* Message list */}
      <div className="flex-1 overflow-y-auto py-4">
        {historyLoading ? (
          <MessageSkeletons />
        ) : allMessages.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-3 px-4 py-16 text-center">
            <span className="flex h-14 w-14 items-center justify-center rounded-full bg-gray-800">
              <svg className="h-7 w-7 text-gray-600" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M8.625 12a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 0 1-2.555-.337A5.972 5.972 0 0 1 5.41 20.97a5.969 5.969 0 0 1-.474-.065 4.48 4.48 0 0 0 .978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25Z" />
              </svg>
            </span>
            <p className="text-sm font-medium text-gray-400">No messages yet</p>
            <p className="text-xs text-gray-600">Say hello to start the conversation.</p>
          </div>
        ) : (
          <div className="px-4">
            {allMessages.map((msg, i) => {
              const isOwn = msg.sender_id === userID
              const avatarUrl = "sender_avatar_url" in msg ? msg.sender_avatar_url : ""
              const isViewOnce = msg.view_once
              const revealedText = revealedMessages.get(msg.id)?.content
              const isTombstone = (msg.tombstone || deletedIds.has(msg.id) || expiredIds.has(msg.id)) && !revealedMessages.has(msg.id)
              const firstInGroup = isFirstInGroup(allMessages, i)
              const lastInGroup = isLastInGroup(allMessages, i)
              const isLive = liveMessages.some((m) => m.id === msg.id) && !historyIdSet.has(msg.id)

              // Date separator
              const showDateSep = i === 0 || !sameCalendarDay(allMessages[i - 1].created_at, msg.created_at)

              return (
                <div key={msg.id}>
                  {showDateSep && (
                    <div className="my-4 flex items-center gap-3">
                      <div className="h-px flex-1 bg-gray-800" />
                      <span className="text-[11px] font-medium text-gray-500">
                        {formatDaySeparator(msg.created_at)}
                      </span>
                      <div className="h-px flex-1 bg-gray-800" />
                    </div>
                  )}

                  <div
                    className={`flex ${isOwn ? "justify-end" : "justify-start"} ${firstInGroup ? "mt-3" : "mt-0.5"} ${isLive ? "animate-message-in" : ""}`}
                  >
                    {/* Avatar placeholder to keep alignment — only shown for last in group */}
                    {!isOwn && (
                      <div className="mr-2 mt-auto flex-none self-end">
                        {lastInGroup ? (
                          avatarUrl ? (
                            <Image src={avatarUrl} alt="" width={28} height={28} className="h-7 w-7 rounded-full object-cover ring-1 ring-gray-700" />
                          ) : (
                            <span className="flex h-7 w-7 items-center justify-center rounded-full bg-gray-700 text-xs text-gray-300 ring-1 ring-gray-600">
                              {(msg.sender_name || "?")[0].toUpperCase()}
                            </span>
                          )
                        ) : (
                          <div className="h-7 w-7" />
                        )}
                      </div>
                    )}

                    <div className={`flex flex-col ${isOwn ? "items-end" : "items-start"}`}>
                      {!isOwn && firstInGroup && (
                        <span className="mb-1 text-xs text-gray-500">{msg.sender_name}</span>
                      )}

                      {isTombstone ? (
                        <>
                          <div className="flex items-center gap-1.5 rounded-2xl border border-dashed border-gray-700 px-3 py-1.5 text-xs text-gray-600">
                            <svg className="h-3 w-3 shrink-0" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                              {isViewOnce ? (
                                <path strokeLinecap="round" strokeLinejoin="round" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
                              ) : (
                                <>
                                  <circle cx="12" cy="12" r="9" />
                                  <path strokeLinecap="round" d="M12 7v5l3 3" />
                                </>
                              )}
                            </svg>
                            {isViewOnce ? "View-once message" : "Message expired"}
                          </div>
                          <span className="mt-1 text-xs text-gray-700">
                            {new Date(msg.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                          </span>
                        </>
                      ) : (
                        <>
                          <div
                            className={`max-w-xs text-sm ${
                              ("thumbnail_url" in msg && msg.thumbnail_url) || (msg.type === "image" && msg.content && !isViewOnce) || (msg.type === "video" && !isViewOnce)
                                ? "overflow-hidden p-0"
                                : isViewOnce && isOwn
                                ? "relative overflow-hidden rounded-br-sm border-2 border-dashed border-white/30 bg-gradient-to-br from-indigo-600 to-purple-700 px-4 py-3 text-white"
                                : msg.expires_at || isViewOnce
                                ? `px-4 py-2 border-2 border-dashed ${isOwn ? "rounded-br-sm bg-indigo-600 text-white border-white/30" : "rounded-bl-sm bg-gray-800 text-gray-100 border-gray-600"}`
                                : `px-4 py-2 ${
                                    isOwn
                                      ? `bg-indigo-600 text-white ${lastInGroup ? "rounded-2xl rounded-br-sm" : firstInGroup ? "rounded-2xl rounded-br-sm" : "rounded-2xl rounded-br-sm"}`
                                      : `bg-gray-800 text-gray-100 ${lastInGroup ? "rounded-2xl rounded-bl-sm" : firstInGroup ? "rounded-2xl rounded-bl-sm" : "rounded-2xl rounded-bl-sm"}`
                                  }`
                            } rounded-2xl`}
                          >
                            {isViewOnce && !isOwn && !revealedText ? (
                              msg.type === "image" || msg.type === "video" || msg.type === "file" ? (
                                <button
                                  onClick={() => handleViewOnce(msg.id)}
                                  className="flex w-44 flex-col items-center gap-3 py-3 transition-transform active:scale-95"
                                >
                                  <div className="relative">
                                    <span className="absolute inset-0 animate-ping rounded-full bg-indigo-400/30" />
                                    <span className="relative flex h-12 w-12 items-center justify-center rounded-full bg-indigo-500/20">
                                      {msg.type === "image" ? (
                                        <svg className="h-6 w-6 text-indigo-300" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24">
                                          <path strokeLinecap="round" strokeLinejoin="round" d="M6.827 6.175A2.31 2.31 0 015.186 7.23c-.38.054-.757.112-1.134.175C2.999 7.58 2.25 8.507 2.25 9.574V18a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9.574c0-1.067-.75-1.994-1.802-2.169a47.865 47.865 0 00-1.134-.175 2.31 2.31 0 01-1.64-1.055l-.822-1.316a2.192 2.192 0 00-1.736-1.039 48.774 48.774 0 00-5.232 0 2.192 2.192 0 00-1.736 1.039l-.821 1.316z" />
                                          <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 12.75a4.5 4.5 0 11-9 0 4.5 4.5 0 019 0zM18.75 10.5h.008v.008h-.008V10.5z" />
                                        </svg>
                                      ) : msg.type === "video" ? (
                                        <svg className="h-6 w-6 text-indigo-300" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24">
                                          <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 10.5l4.72-4.72a.75.75 0 011.28.53v11.38a.75.75 0 01-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 002.25-2.25v-9a2.25 2.25 0 00-2.25-2.25h-9A2.25 2.25 0 002.25 7.5v9a2.25 2.25 0 002.25 2.25z" />
                                        </svg>
                                      ) : (
                                        <svg className="h-6 w-6 text-indigo-300" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24">
                                          <path strokeLinecap="round" strokeLinejoin="round" d="M18.375 12.739l-7.693 7.693a4.5 4.5 0 01-6.364-6.364l10.94-10.94A3 3 0 1119.5 7.372L8.552 18.32m.009-.01l-.01.01m5.699-9.941l-7.81 7.81a1.5 1.5 0 002.112 2.13" />
                                        </svg>
                                      )}
                                    </span>
                                  </div>
                                  <span className="text-xs font-medium text-gray-300">
                                    {msg.type === "image" ? "Tap to view photo" : msg.type === "video" ? "Tap to view video" : "Tap to open file"}
                                  </span>
                                </button>
                              ) : (
                                <button
                                  onClick={() => handleViewOnce(msg.id)}
                                  className="flex items-center gap-2 px-1 py-0.5 transition-transform active:scale-95"
                                >
                                  <div className="relative flex-none">
                                    <span className="absolute inset-0 animate-ping rounded-full bg-indigo-400/30" />
                                    <span className="relative flex h-6 w-6 items-center justify-center rounded-full bg-indigo-500/20">
                                      <svg className="h-3.5 w-3.5 text-indigo-300" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 8.25h9m-9 3H12m-9.75 1.51c0 1.6 1.123 2.994 2.707 3.227 1.129.166 2.27.293 3.423.379.35.026.67.21.865.501L12 21l2.755-4.133a1.14 1.14 0 01.865-.501 48.172 48.172 0 003.423-.379c1.584-.233 2.707-1.626 2.707-3.228V6.741c0-1.602-1.123-2.995-2.707-3.228A48.394 48.394 0 0012 3c-2.392 0-4.744.175-7.043.513C3.373 3.746 2.25 5.14 2.25 6.741v6.018z" />
                                      </svg>
                                    </span>
                                  </div>
                                  <span className="text-xs font-medium text-gray-300">Tap to read</span>
                                </button>
                              )
                            ) : isViewOnce && isOwn ? (
                              <span className="flex items-center gap-2 text-sm font-medium text-white/75">
                                <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                                  <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                                  <path strokeLinecap="round" strokeLinejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                                </svg>
                                {msg.type === "image" ? "Photo · View once" : msg.type === "video" ? "Video · View once" : msg.type === "file" ? "File · View once" : "View once"}
                              </span>
                            ) : revealedText ? (
                              <span className="italic opacity-80">{revealedText}</span>
                            ) : msg.type === "image" && msg.content ? (
                              <button
                                onClick={() => setMediaModal({ url: msg.content, type: "image" })}
                                className="block overflow-hidden rounded-2xl transition-transform active:scale-95"
                              >
                                <Image
                                  src={("thumbnail_url" in msg && msg.thumbnail_url) ? msg.thumbnail_url : msg.content}
                                  alt="image"
                                  width={240}
                                  height={180}
                                  className="max-h-60 w-auto object-cover transition-opacity hover:opacity-90"
                                />
                              </button>
                            ) : msg.type === "video" && msg.content ? (
                              <button
                                onClick={() => setMediaModal({ url: msg.content, type: "video" })}
                                className={`flex items-center gap-2 px-4 py-2 transition-transform active:scale-95 ${isOwn ? "text-indigo-200 hover:text-white" : "text-indigo-400 hover:text-indigo-300"}`}
                              >
                                <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
                                  <path d="M8 5v14l11-7z" />
                                </svg>
                                <span className="underline">Play video</span>
                              </button>
                            ) : msg.type === "file" && msg.content ? (
                              <a
                                href={msg.content}
                                target="_blank"
                                rel="noopener noreferrer"
                                className={`flex items-center gap-2 ${isOwn ? "text-indigo-200 hover:text-white" : "text-indigo-400 hover:text-indigo-300"}`}
                              >
                                <span>📎</span>
                                <span className="truncate underline">{msg.content.split("/").pop() ?? "attachment"}</span>
                              </a>
                            ) : (
                              msg.content
                            )}
                          </div>

                          <div className="mt-1 flex items-center gap-1.5">
                            {msg.expires_at && !isViewOnce && (
                              <span className={`flex items-center gap-1 text-xs font-medium ${expiryColorClass(msg.expires_at, now)}`}>
                                <svg className="h-3 w-3" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                                  <circle cx="12" cy="12" r="9" />
                                  <path strokeLinecap="round" d="M12 7v5l3 3" />
                                </svg>
                                {formatExpiry(msg.expires_at, now)}
                              </span>
                            )}
                            <span className="text-xs text-gray-600">
                              {new Date(msg.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                            </span>
                            {isOwn && (
                              <svg className="h-3.5 w-3.5 text-gray-600" fill="none" stroke="currentColor" strokeWidth="2.5" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" d="M4 12.5l5 5L20 6" />
                              </svg>
                            )}
                          </div>
                          {msg.id === lastSeenOwnMsgId && (
                            <span className="mt-0.5 text-xs text-indigo-400">Seen</span>
                          )}
                        </>
                      )}
                    </div>
                  </div>
                </div>
              )
            })}
            <div ref={bottomRef} />
          </div>
        )}
      </div>

      {/* Typing indicator */}
      {(() => {
        const typers = [...typingUsers.entries()]
          .filter(([id]) => id !== userID)
          .map(([, { displayName }]) => displayName || "Someone")
        if (typers.length === 0) return null
        return (
          <div className="px-4 py-1 text-xs text-gray-400">
            {typers.join(", ")} {typers.length === 1 ? "is" : "are"} typing…
          </div>
        )
      })()}

      {/* Input */}
      <div className="border-t border-gray-800 bg-gray-900 px-4 py-3">
        {ephemeral !== "off" && (
          <div className="mb-2 flex items-center gap-1.5 text-xs text-amber-400">
            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
              {ephemeral === "view_once" ? (
                <>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  <path strokeLinecap="round" strokeLinejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                </>
              ) : (
                <>
                  <circle cx="12" cy="12" r="9" />
                  <path strokeLinecap="round" d="M12 7v5l3 3" />
                </>
              )}
            </svg>
            {EPHEMERAL_LABELS[ephemeral]}
          </div>
        )}

        <div className="relative flex items-center gap-3">
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*,video/*,.pdf,.doc,.docx,.txt,.zip"
            className="hidden"
            onChange={handleFileChange}
          />

          {/* Attach button */}
          <button
            onClick={() => fileInputRef.current?.click()}
            disabled={!connected || uploading}
            aria-label="Attach file"
            title="Attach file"
            className="flex-none rounded-full p-2 text-gray-400 hover:bg-gray-800 hover:text-gray-200 disabled:opacity-40"
          >
            {uploading ? (
              <svg className="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
              </svg>
            ) : (
              <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13" />
              </svg>
            )}
          </button>

          {/* Ephemeral mode button */}
          <div className="relative flex-none">
            <button
              onClick={() => setShowEphemeralMenu((v) => !v)}
              disabled={!connected}
              aria-label="Ephemeral message"
              title="Ephemeral message"
              className={`rounded-full p-2 transition-colors disabled:opacity-40 ${
                ephemeral !== "off"
                  ? "text-amber-400 hover:bg-amber-400/10"
                  : "text-gray-400 hover:bg-gray-800 hover:text-gray-200"
              }`}
            >
              {ephemeral === "view_once" ? (
                <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  <path strokeLinecap="round" strokeLinejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                </svg>
              ) : (
                <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                  <circle cx="12" cy="12" r="9" />
                  <path strokeLinecap="round" d="M12 7v5l3 3" />
                </svg>
              )}
            </button>

            {showEphemeralMenu && (
              <div className="absolute bottom-full left-0 mb-2 w-48 overflow-hidden rounded-xl border border-gray-700 bg-gray-900 shadow-xl">
                <p className="px-4 py-2 text-xs text-gray-500">Applies to messages &amp; attachments</p>
                <div className="border-t border-gray-700/60" />
                {(["off", "view_once", "15m", "30m", "1h", "6h", "12h", "24h"] as EphemeralMode[]).map((mode) => (
                  <button
                    key={mode}
                    onClick={() => { setEphemeral(mode); setShowEphemeralMenu(false) }}
                    className={`flex w-full items-center px-4 py-2.5 text-left text-sm transition-colors hover:bg-gray-800 ${
                      ephemeral === mode ? "text-amber-400" : "text-gray-300"
                    }`}
                  >
                    {EPHEMERAL_LABELS[mode]}
                  </button>
                ))}
              </div>
            )}
          </div>

          <label htmlFor="message-input" className="sr-only">Message</label>
          <input
            id="message-input"
            type="text"
            placeholder="Message…"
            value={input}
            onChange={(e) => {
              setInput(e.target.value)
              if (!typingThrottleRef.current) {
                sendTyping()
                typingThrottleRef.current = setTimeout(() => {
                  typingThrottleRef.current = null
                }, 2000)
              }
            }}
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
