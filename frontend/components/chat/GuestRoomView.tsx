"use client"

import { useEffect, useMemo, useState } from "react"
import { useRouter } from "@/i18n/navigation"
import { useTranslations } from "next-intl"

import type { AnyMessage } from "@/types/chat"
import { useGuestChat } from "@/hooks/useGuestChat"
import PublicRoomShell from "@/components/chat/PublicRoomShell"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface GuestRoomViewProps {
  roomId: string
  sessionId: string
  /** Called when the guest session is no longer valid (expired/invalid). */
  onInvalidSession?: () => void
}

export default function GuestRoomView({ roomId, sessionId, onInvalidSession }: GuestRoomViewProps) {
  const t = useTranslations("guestRooms")
  const router = useRouter()
  const { messages: liveMessages, deletedIds, connected, send, sendTyping, typingUsers, participantEvents, isKicked, isMuted } = useGuestChat(roomId, sessionId, onInvalidSession)
  const [history, setHistory] = useState<AnyMessage[]>([])
  const [historyLoaded, setHistoryLoaded] = useState(false)
  const [roomName, setRoomName] = useState("")

  useEffect(() => {
    fetch(`${API_URL}/guest/rooms`)
      .then((r) => (r.ok ? r.json() : []))
      .then((rooms: { id: string; name: string }[]) => {
        const room = rooms.find((r) => r.id === roomId)
        if (room) setRoomName(room.name)
      })
      .catch(() => {})
  }, [roomId])

  useEffect(() => {
    fetch(`${API_URL}/chat/rooms/${roomId}/messages?limit=50`)
      .then((r) => (r.ok ? r.json() : []))
      .then((msgs: AnyMessage[]) => { setHistory(Array.isArray(msgs) ? msgs : []); setHistoryLoaded(true) })
      .catch(() => setHistoryLoaded(true))
  }, [roomId])

  // Combine history + live messages, deduplicating by id (a message can arrive
  // more than once on the socket — e.g. on reconnect or a StrictMode remount).
  const allMessages = useMemo((): AnyMessage[] => {
    const seen = new Set<string>()
    const out: AnyMessage[] = []
    for (const m of [...history, ...liveMessages]) {
      if (!seen.has(m.id)) { seen.add(m.id); out.push(m) }
    }
    return out
  }, [history, liveMessages])

  const typingNames = useMemo(
    () => [...typingUsers.values()].map((v) => v.displayName).filter(Boolean),
    [typingUsers],
  )

  return (
    <PublicRoomShell
      roomId={roomId}
      roomName={roomName}
      messages={allMessages}
      historyLoaded={historyLoaded}
      connected={connected}
      deletedIds={deletedIds}
      typingNames={typingNames}
      participantEvents={participantEvents}
      isOwn={(senderId) => senderId === sessionId}
      headerBadge={t("guestBadge")}
      confirmOnLeave
      isKicked={isKicked}
      isMuted={isMuted}
      onBack={() => router.push("/rooms")}
      onSend={(content) => send(content)}
      onTyping={sendTyping}
    />
  )
}
