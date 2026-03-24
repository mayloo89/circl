"use client"

import { useCallback, useEffect, useRef, useState } from "react"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
// Derive the WebSocket URL from the HTTP URL: http → ws, https → wss.
const WS_URL = API_URL.replace(/^http/, "ws")

export interface ChatMessage {
  type: string
  id: string
  room_id: string
  sender_id: string
  sender_name: string
  sender_avatar_url: string
  content: string
  thumbnail_url?: string
  view_once: boolean
  tombstone?: boolean
  expires_at?: string
  created_at: string
}

export interface SendOpts {
  viewOnce?: boolean
  ttl?: string // "15m" | "30m" | "1h" | "6h" | "12h" | "24h"
}

export interface TypingUser {
  userId: string
  displayName: string
}

/** Maps userId → timestamp (ms) of their last read event. */
export type ReadReceipts = Map<string, number>

const chatMessageTypes = new Set(["text", "image", "video", "file"])

/**
 * Manages the WebSocket connection for a single chat room.
 *
 * - Connects when both `roomId` and `token` are available.
 * - Reconnects automatically with exponential backoff on error.
 * - Returns the live message list, connection state, send functions,
 *   a set of IDs for messages deleted via message_deleted events,
 *   the current typing users, and a sendTyping function.
 */
export function useChat(roomId: string | null, token: string | undefined) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [deletedIds, setDeletedIds] = useState<Set<string>>(new Set())
  const [connected, setConnected] = useState(false)
  const [typingUsers, setTypingUsers] = useState<Map<string, { displayName: string; at: number }>>(new Map())
  const [readReceipts, setReadReceipts] = useState<ReadReceipts>(new Map())
  const wsRef = useRef<WebSocket | null>(null)
  const retryDelayRef = useRef(1000)
  const cancelledRef = useRef(false)

  const send = useCallback((content: string, opts?: SendOpts) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        type: "message",
        content,
        ...(opts?.viewOnce && { view_once: true }),
        ...(opts?.ttl && { ttl: opts.ttl }),
      }))
    }
  }, [])

  const sendAttachment = useCallback((uploadId: string, url: string, mimeType: string, opts?: SendOpts) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        type: "attachment",
        content: url,
        mime_type: mimeType,
        upload_id: uploadId,
        ...(opts?.viewOnce && { view_once: true }),
        ...(opts?.ttl && { ttl: opts.ttl }),
      }))
    }
  }, [])

  const sendTyping = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: "typing" }))
    }
  }, [])

  useEffect(() => {
    if (!roomId || !token) return

    cancelledRef.current = false

    function connect() {
      if (cancelledRef.current) return

      const ws = new WebSocket(`${WS_URL}/chat/rooms/${roomId}/ws?token=${token}`)
      wsRef.current = ws

      ws.onopen = () => {
        setConnected(true)
        retryDelayRef.current = 1000 // reset backoff
      }

      ws.onclose = () => {
        setConnected(false)
        if (!cancelledRef.current) {
          const delay = retryDelayRef.current
          retryDelayRef.current = Math.min(delay * 2, 30_000)
          setTimeout(connect, delay)
        }
      }

      ws.onerror = () => {
        ws.close()
      }

      ws.onmessage = (e) => {
        try {
          const frame = JSON.parse(e.data)
          if (frame.event === "message_deleted" && frame.id) {
            // Keep the message in `messages` so the page can render a tombstone
            // in its original position. Only track the ID as deleted.
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
          } else if (frame.type && chatMessageTypes.has(frame.type)) {
            setMessages((prev) => [...prev, frame as ChatMessage])
          }
        } catch {
          // ignore malformed frames
        }
      }
    }

    connect()

    return () => {
      cancelledRef.current = true
      wsRef.current?.close()
      setConnected(false)
      setMessages([])
      setDeletedIds(new Set())
      setTypingUsers(new Map())
      setReadReceipts(new Map())
    }
  }, [roomId, token])

  // Clear stale typing entries (older than 3 s) on a 1 s interval.
  useEffect(() => {
    if (!roomId || !token) return
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
  }, [roomId, token])

  return { messages, deletedIds, connected, send, sendAttachment, sendTyping, typingUsers, readReceipts }
}
