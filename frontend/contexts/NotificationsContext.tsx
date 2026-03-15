"use client"

import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react"
import { useSession } from "next-auth/react"

import { useNotifications, type ContactEvent } from "@/hooks/useNotifications"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

type EventListener = (e: ContactEvent) => void

interface NotificationsContextValue {
  /** Number of incoming pending contact requests for the current user. */
  pendingCount: number
  /** Re-fetches the pending count from the server. Call after local accept/decline. */
  refreshPendingCount: () => void
  /**
   * Subscribes to real-time contact events. Returns an unsubscribe function.
   * Use this instead of opening a new SSE connection — there is exactly one
   * connection per authenticated session, managed by this context.
   *
   * @example
   * useEffect(() => subscribe((e) => { ... }), [subscribe])
   */
  subscribe: (listener: EventListener) => () => void
}

const NotificationsContext = createContext<NotificationsContextValue>({
  pendingCount: 0,
  refreshPendingCount: () => {},
  subscribe: () => () => {},
})

export function NotificationsProvider({ children }: { children: React.ReactNode }) {
  const { data: session, status } = useSession()
  const [pendingCount, setPendingCount] = useState(0)
  const token = session?.accessToken

  // Listener registry — never triggers re-renders when mutated.
  const listenersRef = useRef<Set<EventListener>>(new Set())

  const subscribe = useCallback((listener: EventListener) => {
    listenersRef.current.add(listener)
    return () => { listenersRef.current.delete(listener) }
  }, [])

  const refreshPendingCount = useCallback(async () => {
    if (!token) return
    try {
      const res = await fetch(`${API_URL}/contacts/pending`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (res.ok) {
        const data = await res.json()
        setPendingCount(data.length)
      }
    } catch {
      // silent — non-critical
    }
  }, [token])

  // Fetch initial count once authenticated.
  useEffect(() => {
    if (status === "authenticated") refreshPendingCount()
  }, [status, refreshPendingCount])

  // Single SSE connection for the entire app.
  // useNotifications keeps onEvent in a ref internally, so this callback is
  // always current and can safely close over refreshPendingCount.
  useNotifications(token, (e) => {
    // Update the nav badge.
    if (e.type === "contact_request") setPendingCount((n) => n + 1)
    if (e.type === "contact_removed") refreshPendingCount()

    // Fan out to all subscribers (e.g. the contacts page).
    listenersRef.current.forEach((listener) => listener(e))
  })

  return (
    <NotificationsContext.Provider value={{ pendingCount, refreshPendingCount, subscribe }}>
      {children}
    </NotificationsContext.Provider>
  )
}

export function useNotificationsContext() {
  return useContext(NotificationsContext)
}
