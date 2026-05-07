"use client"

import { useEffect } from "react"
import { useSession } from "next-auth/react"
import { useParams } from "next/navigation"

import { useRouter } from "@/i18n/navigation"
import RoomView from "@/components/chat/RoomView"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

/**
 * DM/group conversation page. Channel rooms are hosted under
 * /chat/channels/[channelId] — if a legacy URL or stale link lands a channel
 * room here, redirect transparently rather than render the wrong shell.
 */
export default function MessagesRoomPage() {
  const params = useParams()
  const roomId = typeof params.roomId === "string" ? params.roomId : null
  const { data: session, status } = useSession()
  const router = useRouter()

  useEffect(() => {
    if (status !== "authenticated" || !session?.accessToken || !roomId) return
    let cancelled = false
    fetch(`${API_URL}/chat/rooms/${roomId}`, {
      headers: { Authorization: `Bearer ${session.accessToken}` },
    })
      .then((r) => (r.ok ? r.json() : null))
      .then((data: { type?: string } | null) => {
        if (cancelled || !data) return
        if (data.type === "channel") router.replace(`/chat/channels/${roomId}`)
      })
      .catch(() => {})
    return () => { cancelled = true }
  }, [status, session?.accessToken, roomId, router])

  return <RoomView roomId={roomId} surface="messages" />
}
