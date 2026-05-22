"use client"

import { useEffect, useRef } from "react"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export type ContactEvent =
  | { type: "connected" }
  | { type: "contact_request"; payload: { contact_id: string; requester_id: string } }
  | { type: "contact_accepted"; payload: { contact_id: string; addressee_id: string } }
  | { type: "contact_removed"; payload: { contact_id: string } }
  | { type: "new_message"; payload: { room_id: string } }
  | { type: "presence_online"; payload: { user_id: string } }
  | { type: "presence_offline"; payload: { user_id: string } }

/**
 * Opens an SSE connection to /notifications/stream and calls onEvent for each
 * message received. Reconnects automatically with exponential backoff on error.
 *
 * A short-lived single-use ticket is fetched from POST /ws-ticket before each
 * connection attempt so the long-lived access token is never embedded in the
 * SSE URL (where it would appear in server access logs and browser history).
 */
export function useNotifications(
  token: string | undefined,
  onEvent: (e: ContactEvent) => void,
): void {
  // Keep a ref so the SSE handler always calls the latest callback without
  // re-creating the EventSource connection every time the parent re-renders.
  const onEventRef = useRef(onEvent)
  useEffect(() => {
    onEventRef.current = onEvent
  })

  useEffect(() => {
    if (!token) return

    const accessToken = token
    let es: EventSource | null = null
    let retryTimeout: ReturnType<typeof setTimeout> | null = null
    let retryDelay = 1000
    let cancelled = false

    async function connect() {
      if (cancelled) return

      // Fetch a single-use ticket — avoids putting the JWT in the SSE URL.
      let ticket: string
      try {
        const resp = await fetch(`${API_URL}/ws-ticket`, {
          method: "POST",
          headers: { Authorization: `Bearer ${accessToken}` },
        })
        if (!resp.ok) throw new Error(`ticket fetch ${resp.status}`)
        const data = (await resp.json()) as { ticket: string }
        ticket = data.ticket
      } catch {
        if (cancelled) return
        retryTimeout = setTimeout(() => {
          retryDelay = Math.min(retryDelay * 2, 30_000)
          void connect()
        }, retryDelay)
        return
      }

      if (cancelled) return
      es = new EventSource(`${API_URL}/notifications/stream?ticket=${ticket}`)

      es.onmessage = (msg) => {
        try {
          const event = JSON.parse(msg.data) as ContactEvent
          onEventRef.current(event)
        } catch {
          // ignore malformed events
        }
      }

      es.onerror = () => {
        es?.close()
        if (cancelled) return
        // Fetch a fresh ticket on each reconnect — tickets are single-use.
        retryTimeout = setTimeout(() => {
          retryDelay = Math.min(retryDelay * 2, 30_000)
          void connect()
        }, retryDelay)
      }

      es.onopen = () => {
        retryDelay = 1000 // reset backoff on successful connection
      }
    }

    void connect()

    return () => {
      cancelled = true
      if (retryTimeout !== null) clearTimeout(retryTimeout)
      es?.close()
    }
  }, [token])
}
