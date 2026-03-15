"use client"

import { useCallback, useEffect, useRef, useState } from "react"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
// Derive the WebSocket URL from the HTTP URL: http → ws, https → wss.
const WS_URL = API_URL.replace(/^http/, "ws")

export interface ChatMessage {
  type: "message"
  id: string
  room_id: string
  sender_id: string
  sender_name: string
  sender_avatar_url: string
  content: string
  created_at: string
}

/**
 * Manages the WebSocket connection for a single chat room.
 *
 * - Connects when both `roomId` and `token` are available.
 * - Reconnects automatically with exponential backoff on error.
 * - Returns the live message list, connection state, and a send function.
 */
export function useChat(roomId: string | null, token: string | undefined) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [connected, setConnected] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const retryDelayRef = useRef(1000)
  const cancelledRef = useRef(false)

  const send = useCallback((content: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: "message", content }))
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
          const msg = JSON.parse(e.data) as ChatMessage
          if (msg.type === "message") {
            setMessages((prev) => [...prev, msg])
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
    }
  }, [roomId, token])

  return { messages, connected, send }
}
