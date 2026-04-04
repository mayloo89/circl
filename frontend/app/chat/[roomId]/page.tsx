"use client"

import { useSession } from "next-auth/react"
import { useParams, useRouter } from "next/navigation"
import { useEffect, useRef, useState } from "react"

import { useNotificationsContext } from "@/contexts/NotificationsContext"
import { useChat, type SendOpts } from "@/hooks/useChat"
import { usePresence, formatLastSeen } from "@/hooks/usePresence"
import { useUpload } from "@/hooks/useUpload"
import type { AnyMessage, EphemeralMode, HistoryMessage } from "@/types/chat"
import { sameCalendarDay, formatDaySeparator, isFirstInGroup, isLastInGroup } from "@/lib/chatHelpers"
import Avatar from "@/components/ui/Avatar"
import ConfirmDialog from "@/components/ui/ConfirmDialog"
import PresenceDot from "@/components/ui/PresenceDot"
import Skeleton from "@/components/ui/Skeleton"
import ChatInput from "@/components/chat/ChatInput"
import DateSeparator from "@/components/chat/DateSeparator"
import Lightbox from "@/components/chat/Lightbox"
import MessageBubble from "@/components/chat/MessageBubble"
import TypingIndicator from "@/components/chat/TypingIndicator"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface RoomSummary {
  id: string
  type: "dm" | "group"
  name: string
  peer_id: string
  peer_username: string
  peer_name: string
  peer_avatar_url: string
  peer_last_read_at?: string
}

// ─── Skeletons ────────────────────────────────────────────────────────────────

function MessageSkeletons() {
  return (
    <div className="space-y-4 px-4 py-4">
      <div className="flex items-end gap-2">
        <Skeleton className="h-7 w-7 flex-none rounded-full" />
        <div className="space-y-1">
          <Skeleton className="h-3 w-16 bg-gray-800/60" />
          <Skeleton className="h-9 w-48 rounded-2xl" />
        </div>
      </div>
      <div className="flex justify-end">
        <Skeleton className="h-9 w-36 rounded-2xl bg-indigo-900/50" />
      </div>
      <div className="flex items-end gap-2">
        <Skeleton className="h-7 w-7 flex-none rounded-full" />
        <Skeleton className="h-9 w-64 rounded-2xl" />
      </div>
      <div className="flex justify-end">
        <Skeleton className="h-9 w-52 rounded-2xl bg-indigo-900/50" />
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
  const [historyBefore, setHistoryBefore] = useState<string | null>(null)
  const [loadingOlderHistory, setLoadingOlderHistory] = useState(false)
  const [room, setRoom] = useState<RoomSummary | null>(null)
  const [ephemeral, setEphemeral] = useState<EphemeralMode>("off")
  const [mediaModal, setMediaModal] = useState<{ url: string; type: string } | null>(null)
  const [revealedMessages, setRevealedMessages] = useState<Map<string, { msg: AnyMessage; content: string }>>(new Map())
  const [blockConfirmOpen, setBlockConfirmOpen] = useState(false)
  const [blockLoading, setBlockLoading] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const topSentinelRef = useRef<HTMLDivElement>(null)
  const scrollContainerRef = useRef<HTMLDivElement>(null)

  const { messages: liveMessages, deletedIds, connected, send, sendAttachment, sendTyping, typingUsers, readReceipts } = useChat(roomId, token)
  const { upload, uploading } = useUpload(token)
  const { clearChatBadge, subscribe } = useNotificationsContext()

  const peerIDs = room?.peer_id ? [room.peer_id] : []
  const presence = usePresence(peerIDs, token, subscribe)

  useEffect(() => { clearChatBadge() }, [clearChatBadge])

  useEffect(() => {
    if (status !== "authenticated" || !token || !roomId) return
    fetch(`${API_URL}/chat/rooms`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => r.json())
      .then((rooms: RoomSummary[]) => { const found = rooms.find((r) => r.id === roomId); if (found) setRoom(found) })
      .catch(() => {})
  }, [status, token, roomId])

  useEffect(() => {
    if (status !== "authenticated" || !token || !roomId) return
    fetch(`${API_URL}/chat/rooms/${roomId}/messages?limit=50`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => r.json())
      .then((data: HistoryMessage[]) => {
        const reversed = [...data].reverse()
        setHistory(reversed)
        // data is newest-first; the last item in `data` is the oldest message
        if (data.length === 50) {
          setHistoryBefore(data[data.length - 1].created_at)
        }
      })
      .catch(() => {})
      .finally(() => setHistoryLoading(false))
  }, [status, token, roomId])

  useEffect(() => {
    if (status !== "authenticated" || !token || !roomId) return
    fetch(`${API_URL}/chat/rooms/${roomId}/read`, { method: "PUT", headers: { Authorization: `Bearer ${token}` } }).catch(() => {})
  }, [status, token, roomId])

  useEffect(() => {
    if (!token || !roomId || liveMessages.length === 0) return
    function markRead() {
      if (document.visibilityState !== "visible") return
      const last = liveMessages[liveMessages.length - 1]
      if (last?.sender_id !== userID) {
        fetch(`${API_URL}/chat/rooms/${roomId}/read`, { method: "PUT", headers: { Authorization: `Bearer ${token}` } }).catch(() => {})
      }
    }
    markRead()
    document.addEventListener("visibilitychange", markRead)
    return () => document.removeEventListener("visibilitychange", markRead)
  }, [liveMessages, roomId, token, userID])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [history, liveMessages])

  // IntersectionObserver: load older messages when the top sentinel enters view
  useEffect(() => {
    if (!token || !roomId || !historyBefore) return
    const el = topSentinelRef.current
    if (!el) return

    const observer = new IntersectionObserver(
      async (entries) => {
        if (!entries[0].isIntersecting || loadingOlderHistory) return

        setLoadingOlderHistory(true)
        const container = scrollContainerRef.current
        const prevScrollHeight = container?.scrollHeight ?? 0

        try {
          const res = await fetch(
            `${API_URL}/chat/rooms/${roomId}/messages?before=${encodeURIComponent(historyBefore)}&limit=50`,
            { headers: { Authorization: `Bearer ${token}` } }
          )
          if (!res.ok) return
          const data: HistoryMessage[] = await res.json()
          if (data.length === 0) {
            setHistoryBefore(null)
            return
          }
          const reversed = [...data].reverse()
          setHistory((prev) => [...reversed, ...prev])
          // oldest message in the batch becomes the next cursor
          setHistoryBefore(data.length === 50 ? data[data.length - 1].created_at : null)

          // Restore scroll position so content doesn't jump
          requestAnimationFrame(() => {
            if (container) {
              container.scrollTop += container.scrollHeight - prevScrollHeight
            }
          })
        } finally {
          setLoadingOlderHistory(false)
        }
      },
      { threshold: 0.1 }
    )
    observer.observe(el)
    return () => observer.disconnect()
  }, [token, roomId, historyBefore, loadingOlderHistory])

  const [now, setNow] = useState(0)
  useEffect(() => {
    const initial = setTimeout(() => setNow(Date.now()), 0)
    const id = setInterval(() => setNow(Date.now()), 60_000)
    return () => { clearTimeout(initial); clearInterval(id) }
  }, [])

  // Combine history + live messages, deduplicating by id.
  const seen = new Set<string>()
  const allMessages: AnyMessage[] = []
  for (const msg of [...history, ...liveMessages]) {
    if (!seen.has(msg.id)) { seen.add(msg.id); allMessages.push(msg) }
  }
  for (const [id, { msg }] of revealedMessages) {
    if (!seen.has(id)) { seen.add(id); allMessages.push(msg) }
  }

  const historyIdSet = new Set(history.map((m) => m.id))
  const expiredIds = new Set(
    allMessages.filter((m) => m.expires_at && new Date(m.expires_at).getTime() <= now).map((m) => m.id),
  )

  const peerId = room?.peer_id ?? ""
  const peerReadAtMs = Math.max(
    room?.peer_last_read_at ? new Date(room.peer_last_read_at).getTime() : 0,
    readReceipts.get(peerId) ?? 0,
  )
  const lastSeenOwnMsgId = peerReadAtMs > 0
    ? [...allMessages].reverse().find((m) => m.sender_id === userID && new Date(m.created_at).getTime() <= peerReadAtMs)?.id
    : undefined

  const typers = [...typingUsers.entries()]
    .filter(([id]) => id !== userID)
    .map(([, { displayName }]) => displayName || "Someone")

  function buildOpts(): SendOpts | undefined {
    if (ephemeral === "off") return undefined
    if (ephemeral === "view_once") return { viewOnce: true }
    return { ttl: ephemeral }
  }

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

  async function handleAttach(file: File) {
    const result = await upload(file, "chat-attachment")
    if (result) sendAttachment(result.upload_id, result.url, file.type, buildOpts())
  }

  async function handleBlock() {
    if (!room?.peer_id || !token) return
    setBlockLoading(true)
    try {
      await fetch(`${API_URL}/contacts/${room.peer_id}/block`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      })
      setBlockConfirmOpen(false)
      router.push("/chat")
    } finally {
      setBlockLoading(false)
    }
  }

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
          type={mediaModal.type as "image" | "video"}
          onClose={() => {
            if (mediaModal.url.startsWith("blob:")) URL.revokeObjectURL(mediaModal.url)
            setMediaModal(null)
          }}
        />
      )}

      {room?.type === "dm" && (
        <ConfirmDialog
          open={blockConfirmOpen}
          title="Block user"
          message={`Block ${room.peer_name}? They will not be able to message you and will be hidden from your results.`}
          confirmLabel="Block"
          loading={blockLoading}
          onConfirm={handleBlock}
          onCancel={() => setBlockConfirmOpen(false)}
        />
      )}

      {/* Header */}
      <div className="flex items-center gap-4 border-b border-gray-800 bg-gray-900 px-4 py-3">
        <button aria-label="Back to messages" onClick={() => router.push("/chat")} className="text-gray-400 hover:text-gray-200">
          ←
        </button>
        {room ? (
          room.type === "dm" ? (
            <>
              <button onClick={() => router.push(`/profile/${room.peer_username || room.peer_id}`)} className="flex flex-1 items-center gap-3 hover:opacity-80">
                <Avatar src={room.peer_avatar_url} name={room.peer_name || "?"} size="md" />
                <div className="flex flex-col text-left">
                  <span className="text-sm font-medium text-white">{room.peer_name}</span>
                  {room.peer_id && (
                    <div className="flex items-center gap-1.5">
                      <PresenceDot online={presence[room.peer_id]?.online ?? false} size="sm" />
                      <span className="text-xs text-gray-400">
                        {presence[room.peer_id]?.online
                          ? "Online"
                          : presence[room.peer_id]?.last_seen_at
                          ? formatLastSeen(presence[room.peer_id].last_seen_at)
                          : "Offline"}
                      </span>
                    </div>
                  )}
                </div>
              </button>
              <button
                type="button"
                aria-label="Block user"
                onClick={() => setBlockConfirmOpen(true)}
                className="shrink-0 text-xs text-gray-600 hover:text-red-400"
              >
                Block
              </button>
            </>
          ) : (
            <div className="flex flex-col">
              <span className="text-sm font-medium text-white">{room.name}</span>
            </div>
          )
        ) : (
          <div className="flex items-center gap-2">
            <span className={`h-2 w-2 rounded-full ${connected ? "bg-green-400" : "bg-gray-600"}`} />
            <span className="text-sm text-gray-300">{connected ? "Connected" : "Connecting…"}</span>
          </div>
        )}
      </div>

      {/* Message list */}
      <div ref={scrollContainerRef} className="flex-1 overflow-y-auto py-4">
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
            {/* Top sentinel: triggers loading of older messages */}
            <div ref={topSentinelRef} className="h-px" />
            {loadingOlderHistory && (
              <div className="flex justify-center py-2">
                <span className="text-xs text-gray-500">Loading older messages…</span>
              </div>
            )}
            {allMessages.map((msg, i) => {
              const isOwn = msg.sender_id === userID
              const revealedText = revealedMessages.get(msg.id)?.content
              const isTombstone = (msg.tombstone || deletedIds.has(msg.id) || expiredIds.has(msg.id)) && !revealedMessages.has(msg.id)
              const firstInGroup = isFirstInGroup(allMessages, i)
              const lastInGroup = isLastInGroup(allMessages, i)
              const isLive = liveMessages.some((m) => m.id === msg.id) && !historyIdSet.has(msg.id)
              const showDateSep = i === 0 || !sameCalendarDay(allMessages[i - 1].created_at, msg.created_at)

              return (
                <div key={msg.id}>
                  {showDateSep && <DateSeparator label={formatDaySeparator(msg.created_at, now)} />}
                  <MessageBubble
                    msg={msg}
                    isOwn={isOwn}
                    firstInGroup={firstInGroup}
                    lastInGroup={lastInGroup}
                    isLive={isLive}
                    isTombstone={isTombstone}
                    revealedText={revealedText}
                    isLastSeenOwn={msg.id === lastSeenOwnMsgId}
                    now={now}
                    onViewOnce={handleViewOnce}
                    onOpenMedia={(url, type) => setMediaModal({ url, type })}
                  />
                </div>
              )
            })}
            <div ref={bottomRef} />
          </div>
        )}
      </div>

      <TypingIndicator typers={typers} />

      <ChatInput
        connected={connected}
        uploading={uploading}
        ephemeral={ephemeral}
        onEphemeralChange={setEphemeral}
        onSend={(content) => send(content, buildOpts())}
        onAttach={handleAttach}
        onTyping={sendTyping}
      />
    </div>
  )
}
