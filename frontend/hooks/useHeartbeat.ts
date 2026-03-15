"use client"

import { useEffect } from "react"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
const INTERVAL_MS = 20_000 // 20 seconds

/**
 * Sends a presence heartbeat every 20 seconds while the tab is visible.
 * Stops when the tab is hidden or the component unmounts.
 */
export function useHeartbeat(token: string | undefined): void {
  useEffect(() => {
    if (!token) return

    async function beat() {
      if (document.visibilityState !== "visible") return
      try {
        await fetch(`${API_URL}/presence/heartbeat`, {
          method: "POST",
          headers: { Authorization: `Bearer ${token}` },
        })
      } catch {
        // silent — non-critical
      }
    }

    // Send immediately on mount.
    beat()

    const id = setInterval(beat, INTERVAL_MS)
    const onVisible = () => { if (document.visibilityState === "visible") beat() }
    document.addEventListener("visibilitychange", onVisible)

    return () => {
      clearInterval(id)
      document.removeEventListener("visibilitychange", onVisible)
    }
  }, [token])
}
