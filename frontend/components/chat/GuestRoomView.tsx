"use client"

import { useEffect, useMemo, useRef, useState } from "react"
import { useTranslations } from "next-intl"
import { useRouter } from "@/i18n/navigation"

import type { AnyMessage } from "@/types/chat"
import { useGuestChat } from "@/hooks/useGuestChat"
import MessageBubble from "@/components/chat/MessageBubble"
import ChatInput from "@/components/chat/ChatInput"
import DateSeparator from "@/components/chat/DateSeparator"
import TypingIndicator from "@/components/chat/TypingIndicator"
import Avatar from "@/components/ui/Avatar"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface GuestRoomViewProps {
  roomId: string
  sessionId: string
}

export default function GuestRoomView({ roomId, sessionId }: GuestRoomViewProps) {
  const t = useTranslations("guestRooms")
  const tr = useTranslations("chatRoom")
  const router = useRouter()
  const { messages: liveMessages, deletedIds, connected, send, sendTyping, typingUsers, participantEvents } = useGuestChat(roomId, sessionId)
  const [history, setHistory] = useState<AnyMessage[]>([])
  const [historyLoaded, setHistoryLoaded] = useState(false)
  const [roomName, setRoomName] = useState("")
  const [now] = useState(() => Date.now())
  const bottomRef = useRef<HTMLDivElement>(null)
  const messageAreaRef = useRef<HTMLDivElement>(null)

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
      .then((msgs: AnyMessage[]) => { setHistory(msgs); setHistoryLoaded(true) })
      .catch(() => setHistoryLoaded(true))
  }, [roomId])

  const allMessages = useMemo((): AnyMessage[] => {
    const seen = new Set(history.map((m) => m.id))
    const merged = [...history, ...liveMessages.filter((m) => !seen.has(m.id))]
    return merged
  }, [history, liveMessages])

  const typingNames = useMemo(() => {
    return [...typingUsers.values()].map((v) => v.displayName).filter(Boolean)
  }, [typingUsers])

  const onlineCount = useMemo(() => {
    const joined = participantEvents.filter((e) => e.type === "join").length
    const left = participantEvents.filter((e) => e.type === "leave").length
    return Math.max(0, joined - left)
  }, [participantEvents])

  useEffect(() => {
    if (bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: "smooth" })
    }
  }, [allMessages.length])

  function handleSend(content: string) {
    send(content)
  }

  function formatDay(dateStr: string) {
    return new Date(dateStr).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })
  }

  let lastDate = ""
  let lastSenderId = ""

  const isGuest = (senderId: string) => senderId.startsWith("guest:")

  return (
    <div className="flex h-screen flex-col bg-gray-950">
      <header className="flex items-center gap-3 border-b border-gray-800 bg-gray-900 px-4 py-3">
        <button
          type="button"
          onClick={() => router.push("/rooms")}
          className="cursor-pointer rounded-full p-1.5 text-gray-400 hover:bg-gray-800 hover:text-gray-200 focus:outline-none focus:ring-2 focus:ring-brand-hover"
          aria-label={t("backToRooms")}
        >
          <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5L8.25 12l7.5-7.5" />
          </svg>
        </button>
        <Avatar name={roomName || roomId} size="sm" color="indigo" />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold text-foreground">{roomName || roomId}</p>
          {onlineCount > 0 && (
            <p className="text-xs text-gray-500">{t("membersOnline", { count: onlineCount })}</p>
          )}
        </div>
        <span className="rounded-full bg-brand-primary/10 px-2.5 py-0.5 text-xs font-medium text-brand-primary">
          {t("guestBadge")}
        </span>
      </header>

      {!historyLoaded ? (
        <div className="flex flex-1 items-center justify-center">
          <p className="text-sm text-gray-500">{tr("loading")}</p>
        </div>
      ) : (
        <div ref={messageAreaRef} className="flex-1 overflow-y-auto px-4 py-4">
          {allMessages.map((msg, i) => {
            const msgDate = formatDay(msg.created_at)
            const showDateSep = msgDate !== lastDate
            lastDate = msgDate

            const firstInGroup = msg.sender_id !== lastSenderId
            const lastInGroup = i === allMessages.length - 1 || allMessages[i + 1]?.sender_id !== msg.sender_id
            lastSenderId = msg.sender_id

            return (
              <div key={msg.id}>
                {showDateSep && <DateSeparator label={msgDate} />}
                <div className="flex items-start gap-2">
                  <MessageBubble
                msg={msg}
                isOwn={msg.sender_id === sessionId}
                firstInGroup={firstInGroup}
                lastInGroup={lastInGroup}
                isLive={false}
                isTombstone={deletedIds.has(msg.id)}
                revealedText={undefined}
                isLastSeenOwn={false}
                now={now}
                    onViewOnce={() => {}}
                    onOpenMedia={() => {}}
                  />
                  {isGuest(msg.sender_id) && firstInGroup && (
                    <span className="mt-1 shrink-0 rounded bg-gray-800 px-1.5 py-0.5 text-[10px] font-medium text-gray-400">
                      {t("guestBadge")}
                    </span>
                  )}
                </div>
              </div>
            )
          })}
          <div ref={bottomRef} />
        </div>
      )}

      <TypingIndicator typers={typingNames} />

      <div className="border-t border-gray-800 bg-gray-900 px-4 py-2">
        <p className="text-center text-xs text-gray-600">{t("textOnlyHint")}</p>
      </div>

      <ChatInput
        connected={connected}
        uploading={false}
        ephemeral="off"
        onEphemeralChange={() => {}}
        onSend={handleSend}
        onAttach={() => {}}
        onTyping={sendTyping}
        disableAttach={true}
        disableEphemeral={true}
      />
    </div>
  )
}
