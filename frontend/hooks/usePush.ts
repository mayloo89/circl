"use client"

import { useCallback, useEffect, useState } from "react"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

function urlBase64ToUint8Array(base64String: string): ArrayBuffer {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4)
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/")
  const rawData = atob(base64)
  const buffer = new Uint8Array(rawData.length)
  for (let i = 0; i < rawData.length; i++) {
    buffer[i] = rawData.charCodeAt(i)
  }
  return buffer.buffer
}

export type PushPermission = "default" | "granted" | "denied"

interface UsePushResult {
  permission: PushPermission
  supported: boolean
  enable: () => Promise<void>
  disable: () => Promise<void>
}

export function usePush(token: string | undefined): UsePushResult {
  const supported =
    typeof window !== "undefined" &&
    "serviceWorker" in navigator &&
    "PushManager" in window

  const [permission, setPermission] = useState<PushPermission>(
    supported ? (Notification.permission as PushPermission) : "denied",
  )

  // Sync permission state when it changes externally (e.g. browser settings).
  useEffect(() => {
    if (!supported) return
    setPermission(Notification.permission as PushPermission)
  }, [supported])

  const enable = useCallback(async () => {
    if (!supported || !token) return

    const result = await Notification.requestPermission()
    setPermission(result as PushPermission)
    if (result !== "granted") return

    try {
      const vapidRes = await fetch(`${API_URL}/push/vapid-public-key`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!vapidRes.ok) return
      const { public_key: key } = await vapidRes.json()

      const reg = await navigator.serviceWorker.register("/sw.js")

      // Wait for the SW to become active. In some browsers (Chromium-based)
      // the SW may be in "installing" or "waiting" state after registration.
      const activeReg = await new Promise<ServiceWorkerRegistration>((resolve) => {
        if (reg.active) { resolve(reg); return }
        const sw = reg.installing ?? reg.waiting
        if (!sw) { resolve(reg); return }
        sw.addEventListener("statechange", function handler() {
          if (sw.state === "activated") {
            sw.removeEventListener("statechange", handler)
            resolve(reg)
          }
        })
      })

      // Unsubscribe any stale subscription before creating a new one — a
      // leftover subscription with a different VAPID key causes InvalidStateError.
      const stale = await activeReg.pushManager.getSubscription()
      if (stale) await stale.unsubscribe()

      const sub = await activeReg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(key),
      })

      const json = sub.toJSON()
      await fetch(`${API_URL}/push/subscribe`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          endpoint: sub.endpoint,
          p256dh: json.keys?.p256dh ?? "",
          auth: json.keys?.auth ?? "",
        }),
      })
    } catch (err) {
      console.error("[push] enable failed:", err)
    }
  }, [supported, token])

  const disable = useCallback(async () => {
    if (!supported || !token) return

    try {
      const reg = await navigator.serviceWorker.ready
      const sub = await reg.pushManager.getSubscription()
      if (sub) {
        await fetch(`${API_URL}/push/subscribe`, {
          method: "DELETE",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({ endpoint: sub.endpoint }),
        })
        await sub.unsubscribe()
      }
    } catch {
      // best-effort
    }
    // Permission cannot be revoked programmatically; reflect as "default" so
    // the bell reappears and lets the user re-enable later.
    setPermission("default")
  }, [supported, token])

  // Auto-register SW and subscribe when already granted.
  useEffect(() => {
    if (!supported || !token || Notification.permission !== "granted") return

    navigator.serviceWorker
      .register("/sw.js")
      .then(() => navigator.serviceWorker.ready)
      .then(async (reg) => {  // reg here is already from .ready — correct
        const existing = await reg.pushManager.getSubscription()
        if (existing) return // already subscribed

        const vapidRes = await fetch(`${API_URL}/push/vapid-public-key`, {
          headers: { Authorization: `Bearer ${token}` },
        })
        if (!vapidRes.ok) return
        const { public_key: key } = await vapidRes.json()

        const sub = await reg.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: urlBase64ToUint8Array(key),
        })
        const json = sub.toJSON()
        await fetch(`${API_URL}/push/subscribe`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({
            endpoint: sub.endpoint,
            p256dh: json.keys?.p256dh ?? "",
            auth: json.keys?.auth ?? "",
          }),
        })
      })
      .catch(() => {})
  }, [supported, token])

  return { permission, supported, enable, disable }
}
