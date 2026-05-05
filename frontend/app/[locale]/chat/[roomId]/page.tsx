"use client"

import { useSession } from "next-auth/react"
import { useParams } from "next/navigation"
import { useRouter } from "@/i18n/navigation"
import { useEffect, useRef, useState } from "react"

import { useTranslations } from "next-intl"
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
import GroupMembersPanel from "@/components/chat/GroupMembersPanel"
import Lightbox from "@/components/chat/Lightbox"
import MessageBubble from "@/components/chat/MessageBubble"
import TypingIndicator from "@/components/chat/TypingIndicator"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface RoomSummary {
  id: string
  type: "dm" | "group" | "channel"
  name: string
  description?: string
  creator_id?: string
  peer_id?: string
  peer_username?: string
  peer_name?: string
  peer_avatar_url?: string
  peer_last_read_at?: string
}

interface MemberProfile {
  user_id: string
  username: string
  display_name: string
  avatar_url: string
  is_admin: boolean
  joined_at?: string
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
        <Skeleton className="h-9 w-36 rounded-2xl bg-brand-wash/50" />
      </div>
      <div className="flex items-end gap-2">
        <Skeleton className="h-7 w-7 flex-none rounded-full" />
        <Skeleton className="h-9 w-64 rounded-2xl" />
      </div>
      <div className="flex justify-end">
        <Skeleton className="h-9 w-52 rounded-2xl bg-brand-wash/50" />
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

  const t = useTranslations("chatRoom")
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
  const [leaveConfirmOpen, setLeaveConfirmOpen] = useState(false)
  const [pendingNav, setPendingNav] = useState<(() => void) | null>(null)

  function requestLeave(url: string) {
    if (room?.type === "channel") {
      setPendingNav(() => () => router.push(url))
      setLeaveConfirmOpen(true)
    } else {
      router.push(url)
    }
  }

  function confirmLeave() {
    setLeaveConfirmOpen(false)
    pendingNav?.()
    setPendingNav(null)
  }
  const [groupPanelOpen, setGroupPanelOpen] = useState(false)
  const [groupName, setGroupName] = useState("")
  const [members, setMembers] = useState<MemberProfile[]>([])
  const [memberSidebarOpen, setMemberSidebarOpen] = useState(false)
  const [memberQuery, setMemberQuery] = useState("")
  const bottomRef = useRef<HTMLDivElement>(null)
  const topSentinelRef = useRef<HTMLDivElement>(null)
  const scrollContainerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const isAtBottomRef = useRef(true)
  const didInitialScrollRef = useRef(false)
  const [showFab, setShowFab] = useState(false)

  const { messages: liveMessages, deletedIds, connected, send, sendAttachment, sendTyping, typingUsers, readReceipts, participantEvents } = useChat(roomId, token)
  const { upload, uploading } = useUpload(token)
  const { clearChatBadge, subscribe } = useNotificationsContext()

  const isMultiRoom = room?.type === "group" || room?.type === "channel"
  const peerIDs = isMultiRoom
    ? members.map((m) => m.user_id).filter((id) => id !== userID)
    : room?.peer_id ? [room.peer_id] : []
  const presence = usePresence(peerIDs, token, subscribe)

  useEffect(() => { clearChatBadge() }, [clearChatBadge])

  // Intercept all navigation while inside a channel to show a leave confirmation.
  useEffect(() => {
    if (room?.type !== "channel") return

    // Browser-level: refresh, tab close, address-bar navigation.
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault()
      e.returnValue = ""
    }
    window.addEventListener("beforeunload", handleBeforeUnload)

    // In-app navigation: intercept <a> clicks in capture phase before Next.js handles them.
    // Skips new-tab links and external URLs so only same-app navigation is blocked.
    const handleClick = (e: MouseEvent) => {
      const anchor = (e.target as Element).closest("a[href]")
      if (!anchor) return
      if (anchor.getAttribute("target") === "_blank") return
      const href = anchor.getAttribute("href") ?? ""
      if (!href || href.startsWith("#")) return
      if (href.startsWith("http") && !href.startsWith(window.location.origin)) return

      e.preventDefault()
      e.stopPropagation()
      // href is the already locale-prefixed rendered href (e.g. "/es/browse").
      // Using next-intl's router.push would double-prefix it to "/es/es/browse".
      setPendingNav(() => () => { window.location.assign(href) })
      setLeaveConfirmOpen(true)
    }

    document.addEventListener("click", handleClick, true)

    return () => {
      window.removeEventListener("beforeunload", handleBeforeUnload)
      document.removeEventListener("click", handleClick, true)
    }
  }, [room?.type, router])

  // Update the members sidebar in real-time from participant_join / participant_leave events.
  useEffect(() => {
    if (participantEvents.length === 0 || room?.type !== "channel") return
    for (const ev of participantEvents) {
      if (ev.type === "join") {
        setMembers((prev) => {
          if (prev.some((m) => m.user_id === ev.userId)) return prev
          return [...prev, { user_id: ev.userId, username: ev.username, display_name: ev.displayName, avatar_url: ev.avatarURL, is_admin: false }]
        })
      } else {
        setMembers((prev) => prev.filter((m) => m.user_id !== ev.userId))
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [participantEvents])

  useEffect(() => {
    if (status !== "authenticated" || !token || !roomId || !room) return
    if (room.type !== "group" && room.type !== "channel") return
    fetch(`${API_URL}/chat/rooms/${roomId}/members`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => r.ok ? r.json() : [])
      .then((data: MemberProfile[]) => setMembers(data))
      .catch(() => {})
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, token, roomId, room?.type])

  useEffect(() => {
    if (status !== "authenticated" || !token || !roomId) return
    // Fetch the full rooms list first (includes peer info for DMs).
    // If the room is not found there (e.g. a channel, which has no room_members row),
    // fall back to GET /chat/rooms/{id} which works for any room type.
    fetch(`${API_URL}/chat/rooms`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => (r.ok ? r.json() : []))
      .then((rooms: RoomSummary[]) => {
        const found = rooms.find((r) => r.id === roomId)
        if (found) {
          setRoom(found)
          if (found.type === "group" || found.type === "channel") setGroupName(found.name)
          if (found.type === "channel") setMemberSidebarOpen(true)
          return
        }
        // Not in list — likely a channel. Fetch it directly.
        return fetch(`${API_URL}/chat/rooms/${roomId}`, { headers: { Authorization: `Bearer ${token}` } })
          .then((r) => r.ok ? r.json() : null)
          .then((data: RoomSummary | null) => {
            if (data) { setRoom(data); if (data.type === "channel") { setGroupName(data.name); setMemberSidebarOpen(true) } }
          })
      })
      .catch(() => {})
  }, [status, token, roomId])

  useEffect(() => {
    if (status !== "authenticated" || !token || !roomId || !room) return
    if (room.type === "channel") {
      setHistoryLoading(false)
      return
    }
    fetch(`${API_URL}/chat/rooms/${roomId}/messages?limit=50`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => (r.ok ? r.json() : []))
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
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, token, roomId, room?.type])

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

  // Track scroll position to drive FAB visibility and smart auto-scroll.
  useEffect(() => {
    const container = scrollContainerRef.current
    if (!container) return
    function onScroll() {
      const { scrollTop, scrollHeight, clientHeight } = container!
      const atBottom = scrollHeight - scrollTop - clientHeight < 80
      isAtBottomRef.current = atBottom
      setShowFab(!atBottom)
    }
    container.addEventListener("scroll", onScroll, { passive: true })
    return () => container.removeEventListener("scroll", onScroll)
  }, [])

  // Scroll to bottom once when history first finishes loading.
  useEffect(() => {
    if (historyLoading || didInitialScrollRef.current) return
    didInitialScrollRef.current = true
    requestAnimationFrame(() => {
      bottomRef.current?.scrollIntoView()
      isAtBottomRef.current = true
      setShowFab(false)
    })
  }, [historyLoading])

  // Auto-scroll on new live messages — only when already at bottom.
  useEffect(() => {
    if (!didInitialScrollRef.current || liveMessages.length === 0) return
    if (isAtBottomRef.current) {
      bottomRef.current?.scrollIntoView({ behavior: "smooth" })
    } else {
      setShowFab(true)
    }
  }, [liveMessages])

  // Focus input when entering the chat room
  useEffect(() => {
    if (!historyLoading) {
      requestAnimationFrame(() => {
        inputRef.current?.focus()
      })
    }
  }, [historyLoading])

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

  function scrollToBottom() {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" })
    isAtBottomRef.current = true
    setShowFab(false)
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
      <div className="flex h-full flex-col bg-gray-950">
        <MessageSkeletons />
      </div>
    )
  }

  return (
    <div className="flex h-full flex-col bg-gray-950">
      {/* Group / channel members side panel */}
      {groupPanelOpen && (room?.type === "group" || room?.type === "channel") && token && userID && roomId && (
        <div className="absolute inset-0 z-30 bg-gray-950">
          <GroupMembersPanel
            roomId={roomId}
            roomName={groupName || room.name}
            roomType={room.type}
            currentUserId={userID}
            token={token}
            onClose={() => setGroupPanelOpen(false)}
            onNameUpdated={(name) => {
              setGroupName(name)
              setRoom((prev) => prev ? { ...prev, name } : prev)
            }}
            onLeft={() => requestLeave(room.type === "channel" ? "/chat/channels" : "/chat")}
          />
        </div>
      )}

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

      <ConfirmDialog
        open={leaveConfirmOpen}
        title={t("leaveChannelTitle")}
        message={t("leaveChannelMessage")}
        confirmLabel={t("leave")}
        onConfirm={confirmLeave}
        onCancel={() => setLeaveConfirmOpen(false)}
      />

      {room?.type === "dm" && (
        <ConfirmDialog
          open={blockConfirmOpen}
          title={t("blockUserTitle")}
          message={t("blockUserMessage", { name: room.peer_name ?? "" })}
          confirmLabel={t("block")}
          loading={blockLoading}
          onConfirm={handleBlock}
          onCancel={() => setBlockConfirmOpen(false)}
        />
      )}

      {/* Header */}
      <div className="flex items-center gap-4 border-b border-gray-800 bg-gray-900 px-4 py-3">
        <button aria-label={t("backToMessages")} onClick={() => requestLeave(room?.type === "channel" ? "/chat/channels" : "/chat")} className="cursor-pointer rounded p-3 text-gray-400 transition-colors hover:text-gray-200 focus:outline-none focus:ring-2 focus:ring-brand-hover">
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
          </svg>
        </button>
        {room ? (
          room.type === "dm" ? (
            <>
              <button onClick={() => router.push(`/profile/${room.peer_username || room.peer_id}`)} className="flex flex-1 cursor-pointer items-center gap-3 transition-opacity hover:opacity-80 focus:outline-none focus:ring-2 focus:ring-brand-hover rounded">
                <Avatar src={room.peer_avatar_url} name={room.peer_name || "?"} size="md" />
                <div className="flex flex-col text-left">
                  <span className="text-sm font-medium text-white">{room.peer_name}</span>
                  {room.peer_id && (
                    <div className="flex items-center gap-1.5">
                      <PresenceDot online={presence[room.peer_id]?.online ?? false} size="sm" />
                      <span className="text-xs text-gray-400">
                        {presence[room.peer_id]?.online
                          ? t("online")
                          : presence[room.peer_id]?.last_seen_at
                          ? formatLastSeen(presence[room.peer_id].last_seen_at)
                          : t("offline")}
                      </span>
                    </div>
                  )}
                </div>
              </button>
              <button
                type="button"
                aria-label={t("blockUserTitle")}
                onClick={() => setBlockConfirmOpen(true)}
                className="shrink-0 cursor-pointer text-xs text-gray-600 transition-colors hover:text-red-400 focus:outline-none focus:ring-2 focus:ring-red-400 rounded"
              >
                {t("block")}
              </button>
            </>
          ) : (
            <>
              <button
                type="button"
                onClick={() => setGroupPanelOpen(true)}
                className="flex flex-1 cursor-pointer items-center gap-3 text-left transition-opacity hover:opacity-80 focus:outline-none focus:ring-2 focus:ring-brand-hover rounded"
                aria-label={room.type === "channel" ? t("channelSettings") : t("groupSettings")}
              >
                <Avatar name={groupName || room.name || "G"} size="md" color="indigo" />
                <div className="flex flex-col">
                  <span className="text-sm font-medium text-white">
                    {room.type === "channel" ? "# " : ""}{groupName || room.name}
                  </span>
                  <span className="text-xs text-gray-500">
                    {members.length > 0
                      ? t("membersCount", { count: members.length })
                      : room.type === "channel" ? t("publicChannel") : t("group")}
                  </span>
                </div>
              </button>
              <button
                type="button"
                aria-label={memberSidebarOpen ? t("hideMembers") : t("showMembers")}
                onClick={() => setMemberSidebarOpen((v) => !v)}
                className={`shrink-0 rounded p-1.5 text-sm transition-colors ${memberSidebarOpen ? "bg-gray-700 text-white" : "text-gray-400 hover:text-white hover:bg-gray-800"}`}
              >
                <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z" />
                </svg>
              </button>
            </>
          )
        ) : (
          <div className="flex items-center gap-2">
            <span className={`h-2 w-2 rounded-full ${connected ? "bg-green-400" : "bg-gray-600"}`} />
            <span className="text-sm text-gray-300">{connected ? t("connected") : t("connecting")}</span>
          </div>
        )}
      </div>

      {/* Body: message area + optional members sidebar */}
      <div className="flex flex-1 overflow-hidden">
        {/* Message column */}
        <div className="relative flex flex-1 flex-col overflow-hidden">
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
                <p className="text-sm font-medium text-gray-400">{t("noMessages")}</p>
                <p className="text-xs text-gray-600">{t("noMessagesDesc")}</p>
              </div>
            ) : (
              <div className="px-4">
                {/* Top sentinel: triggers loading of older messages */}
                <div ref={topSentinelRef} className="h-px" />
                {loadingOlderHistory && (
                  <div className="flex justify-center py-2">
                    <span className="text-xs text-gray-500">{t("loadingOlder")}</span>
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

          {showFab && (
            <button
              onClick={scrollToBottom}
              className="absolute bottom-24 right-4 z-10 flex h-10 w-10 items-center justify-center rounded-full bg-brand-primary shadow-lg transition-colors hover:bg-brand-hover focus:outline-none focus:ring-2 focus:ring-brand-hover focus:ring-offset-2 focus:ring-offset-gray-950"
              aria-label={t("scrollToBottom")}
            >
              <svg className="h-5 w-5 text-white" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
              </svg>
            </button>
          )}
          <TypingIndicator typers={typers} />

          <ChatInput
            connected={connected}
            uploading={uploading}
            ephemeral={ephemeral}
            onEphemeralChange={setEphemeral}
            onSend={(content) => send(content, buildOpts())}
            onAttach={handleAttach}
            onTyping={sendTyping}
            inputRef={inputRef}
            disableAttach={room?.type === "channel"}
            disableEphemeral={room?.type === "channel"}
          />
        </div>

        {/* Members sidebar — only for group / channel */}
        {isMultiRoom && memberSidebarOpen && (
          <aside className="hidden sm:flex w-52 shrink-0 flex-col border-l border-gray-800 bg-gray-900">
            <div className="px-3 pt-3 pb-2 space-y-2">
              <p className="text-[11px] font-semibold uppercase tracking-wider text-gray-500">
                {t("membersTitle", { count: members.length })}
              </p>
              {room?.type === "channel" && (
                <input
                  type="text"
                  value={memberQuery}
                  onChange={(e) => setMemberQuery(e.target.value)}
                  placeholder={t("filterMembers")}
                  className="w-full rounded bg-gray-800 px-2 py-1 text-xs text-gray-200 placeholder-gray-600 outline-none focus:ring-1 focus:ring-brand-hover"
                />
              )}
            </div>
            <ul className="overflow-y-auto">
              {[...members]
                .filter((m) => {
                  if (m.user_id === userID) return false
                  if (!memberQuery) return true
                  const name = (m.display_name || m.username).toLowerCase()
                  return name.startsWith(memberQuery.toLowerCase())
                })
                .sort((a, b) => {
                  const aOnline = presence[a.user_id]?.online ?? false
                  const bOnline = presence[b.user_id]?.online ?? false
                  if (aOnline !== bOnline) return aOnline ? -1 : 1
                  return (a.display_name || a.username).localeCompare(b.display_name || b.username)
                })
                .map((m) => {
                  const online = presence[m.user_id]?.online ?? false
                  return (
                    <li key={m.user_id} className="flex items-center gap-2 px-3 py-2 hover:bg-gray-800/50">
                      <div className="relative shrink-0">
                        <Avatar src={m.avatar_url} name={m.display_name || m.username || "?"} size="xs" />
                        <span
                          className={`absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full ring-2 ring-gray-900 ${online ? "bg-green-400" : "bg-gray-600"}`}
                          aria-hidden="true"
                        />
                      </div>
                      <div className="min-w-0">
                        {m.username ? (
                          <a
                            href={`/profile/${m.username}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className={`block truncate text-xs font-medium hover:underline ${online ? "text-white" : "text-gray-400"}`}
                          >
                            {m.display_name || m.username}
                          </a>
                        ) : (
                          <p className={`truncate text-xs font-medium ${online ? "text-white" : "text-gray-400"}`}>
                            {m.display_name || m.user_id}
                          </p>
                        )}
                        {m.is_admin && (
                          <p className="text-[10px] text-brand-muted">{t("admin")}</p>
                        )}
                      </div>
                    </li>
                  )
                })}
            </ul>
          </aside>
        )}
      </div>
    </div>
  )
}
