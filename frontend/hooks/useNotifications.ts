"use client"

import { useEffect } from "react"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export type ContactEvent =
  | { type: "connected" }
  | { type: "contact_request"; payload: { contact_id: string; requester_id: string } }
  | { type: "contact_accepted"; payload: { contact_id: string; addressee_id: string } }

/**
 * Opens an SSE connection to /notifications/stream and calls onEvent for each
 * message received. Reconnects automatically with exponential backoff on error.
 *
 * The token is passed as a query parameter because the browser EventSource API
 * does not support custom request headers.
 */
export function useNotifications(
  token: string | undefined,
  onEvent: (e: ContactEvent) => void,
): void {
  useEffect(() => {
    if (!token) return

    let es: EventSource | null = null
    let retryTimeout: ReturnType<typeof setTimeout> | null = null
    let retryDelay = 1000
    let cancelled = false

    function connect() {
      if (cancelled) return
      es = new EventSource(`${API_URL}/notifications/stream?token=${token}`)

      es.onmessage = (msg) => {
        try {
          const event = JSON.parse(msg.data) as ContactEvent
          onEvent(event)
        } catch {
          // ignore malformed events
        }
      }

      es.onerror = () => {
        es?.close()
        if (cancelled) return
        retryTimeout = setTimeout(() => {
          retryDelay = Math.min(retryDelay * 2, 30_000)
          connect()
        }, retryDelay)
      }

      es.onopen = () => {
        retryDelay = 1000 // reset backoff on successful connection
      }
    }

    connect()

    return () => {
      cancelled = true
      if (retryTimeout !== null) clearTimeout(retryTimeout)
      es?.close()
    }
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps
}
