"use client"

import { useCallback, useEffect, useRef, useState } from "react"

import type { ChatMessage, ParticipantEvent, ReadReceipts } from "./useChat"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
const WS_URL = API_URL.replace(/^http/, "ws")

const chatMessageTypes = new Set(["text", "image", "video", "file", "album_share", "system"])

export function useGuestChat(roomId: string | null, sessionId: string | undefined) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [deletedIds, setDeletedIds] = useState<Set<string>>(new Set())
  const [connected, setConnected] = useState(false)
  const [typingUsers, setTypingUsers] = useState<Map<string, { displayName: string; at: number }>>(new Map())
  const [readReceipts, setReadReceipts] = useState<ReadReceipts>(new Map())
  const [participantEvents, setParticipantEvents] = useState<ParticipantEvent[]>([])
  const wsRef = useRef<WebSocket | null>(null)
  const retryDelayRef = useRef(1000)
  const cancelledRef = useRef(false)

  const send = useCallback((content: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: "message", content }))
    }
  }, [])

  const sendTyping = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: "typing" }))
    }
  }, [])

  useEffect(() => {
    if (!roomId || !sessionId) return

    cancelledRef.current = false

    async function connect() {
      if (cancelledRef.current) return

      let ticket: string
      try {
        const res = await fetch(`${API_URL}/guest/ws-ticket`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ session_id: sessionId, room_id: roomId }),
        })
        if (!res.ok) {
          if (!cancelledRef.current) {
            const delay = retryDelayRef.current
            retryDelayRef.current = Math.min(delay * 2, 30_000)
            setTimeout(() => void connect(), delay)
          }
          return
        }
        const data = await res.json() as { ticket: string }
        ticket = data.ticket
      } catch {
        if (!cancelledRef.current) {
          const delay = retryDelayRef.current
          retryDelayRef.current = Math.min(delay * 2, 30_000)
          setTimeout(() => void connect(), delay)
        }
        return
      }

      if (cancelledRef.current) return

      const ws = new WebSocket(`${WS_URL}/chat/rooms/${roomId}/ws?ticket=${ticket}`)
      wsRef.current = ws

      ws.onopen = () => {
        setConnected(true)
        retryDelayRef.current = 1000
      }

      ws.onclose = () => {
        setConnected(false)
        if (!cancelledRef.current) {
          const delay = retryDelayRef.current
          retryDelayRef.current = Math.min(delay * 2, 30_000)
          setTimeout(() => void connect(), delay)
        }
      }

      ws.onerror = () => { ws.close() }

      ws.onmessage = (e) => {
        try {
          const frame = JSON.parse(e.data)
          if (frame.event === "message_deleted" && frame.id) {
            setDeletedIds((prev) => new Set([...prev, frame.id as string]))
          } else if (frame.event === "read_receipt" && frame.user_id && frame.read_at) {
            const ts = new Date(frame.read_at as string).getTime()
            if (!isNaN(ts)) {
              setReadReceipts((prev) => {
                const next = new Map(prev)
                next.set(frame.user_id as string, ts)
                return next
              })
            }
          } else if (frame.event === "typing" && frame.user_id) {
            setTypingUsers((prev) => {
              const next = new Map(prev)
              next.set(frame.user_id as string, {
                displayName: (frame.display_name as string) || "",
                at: Date.now(),
              })
              return next
            })
          } else if (frame.event === "participant_join" && frame.user_id) {
            setParticipantEvents((prev) => [...prev, {
              type: "join",
              userId: frame.user_id as string,
              username: (frame.username as string) || "",
              displayName: (frame.display_name as string) || "",
              avatarURL: (frame.avatar_url as string) || "",
              isGuest: Boolean(frame.is_guest),
            }])
          } else if (frame.event === "participant_leave" && frame.user_id) {
            setParticipantEvents((prev) => [...prev, {
              type: "leave",
              userId: frame.user_id as string,
              username: "",
              displayName: "",
              avatarURL: "",
              isGuest: Boolean(frame.is_guest),
            }])
          } else if (frame.type && chatMessageTypes.has(frame.type)) {
            setMessages((prev) => [...prev, frame as ChatMessage])
          }
        } catch { /* ignore malformed frames */ }
      }
    }

    void connect()

    return () => {
      cancelledRef.current = true
      wsRef.current?.close()
      setConnected(false)
      setMessages([])
      setDeletedIds(new Set())
      setTypingUsers(new Map())
      setReadReceipts(new Map())
      setParticipantEvents([])
    }
  }, [roomId, sessionId])

  useEffect(() => {
    if (!roomId || !sessionId) return
    const id = setInterval(() => {
      const cutoff = Date.now() - 3000
      setTypingUsers((prev) => {
        const stale = [...prev.entries()].filter(([, v]) => v.at < cutoff)
        if (stale.length === 0) return prev
        const next = new Map(prev)
        for (const [k] of stale) next.delete(k)
        return next
      })
    }, 1000)
    return () => clearInterval(id)
  }, [roomId, sessionId])

  return { messages, deletedIds, connected, send, sendTyping, typingUsers, readReceipts, participantEvents }
}
