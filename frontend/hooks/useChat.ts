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
  view_once: boolean
  tombstone?: boolean
  expires_at?: string
  created_at: string
}

export interface SendOpts {
  viewOnce?: boolean
  ttl?: string // "1h" | "24h" | "7d"
}

const chatMessageTypes = new Set(["text", "image", "video", "file"])

/**
 * Manages the WebSocket connection for a single chat room.
 *
 * - Connects when both `roomId` and `token` are available.
 * - Reconnects automatically with exponential backoff on error.
 * - Returns the live message list, connection state, send functions, and
 *   a set of IDs for messages deleted via message_deleted events.
 */
export function useChat(roomId: string | null, token: string | undefined) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [deletedIds, setDeletedIds] = useState<Set<string>>(new Set())
  const [connected, setConnected] = useState(false)
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
    }
  }, [roomId, token])

  return { messages, deletedIds, connected, send, sendAttachment }
}
