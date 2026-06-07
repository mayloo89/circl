"use client"

import { useEffect, useMemo, useState } from "react"
import { useSession } from "next-auth/react"
import { useRouter } from "@/i18n/navigation"

import type { AnyMessage } from "@/types/chat"
import { useChat } from "@/hooks/useChat"
import PublicRoomShell from "@/components/chat/PublicRoomShell"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface RegisteredPublicRoomViewProps {
  roomId: string
  token: string
  userID: string
}

export default function RegisteredPublicRoomView({ roomId, token, userID }: RegisteredPublicRoomViewProps) {
  const router = useRouter()
  const { data: session } = useSession()
  const isAdmin = session?.role === "admin" || session?.role === "super_admin"
  const { messages: liveMessages, deletedIds, connected, send, sendTyping, participantEvents, isKicked, isMuted } = useChat(roomId, token)
  const [history, setHistory] = useState<AnyMessage[]>([])
  const [historyLoaded, setHistoryLoaded] = useState(false)
  const [roomName, setRoomName] = useState("")

  useEffect(() => {
    fetch(`${API_URL}/chat/rooms/${roomId}`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => (r.ok ? r.json() : null))
      .then((room: { name?: string } | null) => { if (room?.name) setRoomName(room.name) })
      .catch(() => {})
  }, [roomId, token])

  useEffect(() => {
    fetch(`${API_URL}/chat/rooms/${roomId}/messages?limit=50`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => (r.ok ? r.json() : []))
      .then((msgs: AnyMessage[]) => { setHistory(Array.isArray(msgs) ? msgs : []); setHistoryLoaded(true) })
      .catch(() => setHistoryLoaded(true))
  }, [roomId, token])

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

  return (
    <PublicRoomShell
      roomId={roomId}
      roomName={roomName}
      token={token}
      messages={allMessages}
      historyLoaded={historyLoaded}
      connected={connected}
      deletedIds={deletedIds}
      participantEvents={participantEvents}
      isOwn={(senderId) => senderId === userID}
      isKicked={isKicked}
      isMuted={isMuted}
      isAdmin={isAdmin}
      viewerId={userID}
      onBack={() => router.push("/chat/channels")}
      onSend={(content) => send(content)}
      onTyping={sendTyping}
    />
  )
}
