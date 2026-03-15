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
  /** Total unread message count across all rooms. Incremented by SSE new_message events. */
  unreadChatCount: number
  /** Resets the unread chat badge to zero. Call when the user enters any chat page. */
  clearChatBadge: () => void
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
  unreadChatCount: 0,
  clearChatBadge: () => {},
  subscribe: () => () => {},
})

export function NotificationsProvider({ children }: { children: React.ReactNode }) {
  const { data: session, status } = useSession()
  const [pendingCount, setPendingCount] = useState(0)
  const [unreadChatCount, setUnreadChatCount] = useState(0)
  const token = session?.accessToken

  const clearChatBadge = useCallback(() => setUnreadChatCount(0), [])

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

  // Fetch initial counts once authenticated.
  useEffect(() => {
    if (status !== "authenticated" || !token) return
    let cancelled = false

    fetch(`${API_URL}/contacts/pending`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => (res.ok ? res.json() : Promise.reject()))
      .then((data: unknown[]) => { if (!cancelled) setPendingCount(data.length) })
      .catch(() => {})

    fetch(`${API_URL}/chat/rooms`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => (res.ok ? res.json() : Promise.reject()))
      .then((rooms: { unread_count: number }[]) => {
        if (!cancelled) {
          setUnreadChatCount(rooms.reduce((sum, r) => sum + r.unread_count, 0))
        }
      })
      .catch(() => {})

    return () => { cancelled = true }
  }, [status, token])

  // Single SSE connection for the entire app.
  // useNotifications keeps onEvent in a ref internally, so this callback is
  // always current and can safely close over refreshPendingCount.
  useNotifications(token, (e) => {
    // Update the nav badges.
    if (e.type === "contact_request") setPendingCount((n) => n + 1)
    if (e.type === "contact_removed") refreshPendingCount()
    if (e.type === "new_message") setUnreadChatCount((n) => n + 1)

    // Fan out to all subscribers (e.g. the contacts page).
    listenersRef.current.forEach((listener) => listener(e))
  })

  return (
    <NotificationsContext.Provider value={{ pendingCount, refreshPendingCount, unreadChatCount, clearChatBadge, subscribe }}>
      {children}
    </NotificationsContext.Provider>
  )
}

export function useNotificationsContext() {
  return useContext(NotificationsContext)
}
