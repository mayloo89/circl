"use client"

import { useEffect, useState } from "react"

import type { ContactEvent } from "@/hooks/useNotifications"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
const POLL_MS = 30_000 // 30 seconds

export interface PresenceInfo {
  user_id: string
  online: boolean
  last_seen_at: string | null
}

export type PresenceMap = Record<string, PresenceInfo>

type SubscribeFn = (listener: (e: ContactEvent) => void) => () => void

/**
 * Polls GET /presence?ids=... every 30 seconds for the given user IDs.
 * Optionally accepts a subscribe function from NotificationsContext to react
 * to presence_online / presence_offline SSE events immediately.
 * Returns a map keyed by user ID for O(1) lookups in render.
 */
export function usePresence(
  userIDs: string[],
  token: string | undefined,
  subscribe?: SubscribeFn,
): PresenceMap {
  const [presence, setPresence] = useState<PresenceMap>({})

  useEffect(() => {
    if (!token || userIDs.length === 0) return

    async function fetch_() {
      try {
        const res = await fetch(
          `${API_URL}/presence?ids=${userIDs.join(",")}`,
          { headers: { Authorization: `Bearer ${token}` } },
        )
        if (!res.ok) return
        const data: PresenceInfo[] = await res.json()
        const map: PresenceMap = {}
        for (const info of data) map[info.user_id] = info
        setPresence(map)
      } catch {
        // silent — non-critical
      }
    }

    fetch_()
    const id = setInterval(fetch_, POLL_MS)
    return () => clearInterval(id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, userIDs.join(",")])

  // React to SSE events for instant updates without waiting for the next poll.
  useEffect(() => {
    if (!subscribe) return
    return subscribe((e) => {
      if (e.type === "presence_online") {
        const { user_id } = e.payload
        setPresence((prev) => {
          if (!prev[user_id]) return prev
          return { ...prev, [user_id]: { ...prev[user_id], online: true } }
        })
      }
      if (e.type === "presence_offline") {
        const { user_id } = e.payload
        setPresence((prev) => {
          if (!prev[user_id]) return prev
          return { ...prev, [user_id]: { ...prev[user_id], online: false } }
        })
      }
    })
  }, [subscribe])

  return presence
}

/**
 * Returns a human-readable "last seen" string for display.
 * Examples: "just now", "2 minutes ago", "3 hours ago", "yesterday"
 */
export function formatLastSeen(lastSeenAt: string | null): string {
  if (!lastSeenAt) return "a while ago"
  const diff = Date.now() - new Date(lastSeenAt).getTime()
  const mins = Math.floor(diff / 60_000)
  if (mins < 1) return "just now"
  if (mins < 60) return `${mins} minute${mins === 1 ? "" : "s"} ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours} hour${hours === 1 ? "" : "s"} ago`
  const days = Math.floor(hours / 24)
  if (days === 1) return "yesterday"
  return `${days} days ago`
}
