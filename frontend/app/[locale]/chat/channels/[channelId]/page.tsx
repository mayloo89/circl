"use client"

import { useEffect } from "react"
import { useSession } from "next-auth/react"
import { useParams } from "next/navigation"

import { useRouter } from "@/i18n/navigation"
import RoomView from "@/components/chat/RoomView"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

/**
 * Channel room page. Lives outside the (messages) layout so it never inherits
 * the DM list pane — channels are deliberately single-pane to make leaving
 * (which permanently removes access to past messages) a conscious action.
 *
 * If a non-channel room id ever lands here (via a stale or hand-typed URL),
 * redirect to the correct DM/group route under (messages).
 */
export default function ChannelRoomPage() {
  const params = useParams()
  const channelId = typeof params.channelId === "string" ? params.channelId : null
  const { data: session, status } = useSession()
  const router = useRouter()

  useEffect(() => {
    if (status !== "authenticated" || !session?.accessToken || !channelId) return
    let cancelled = false
    fetch(`${API_URL}/chat/rooms/${channelId}`, {
      headers: { Authorization: `Bearer ${session.accessToken}` },
    })
      .then((r) => (r.ok ? r.json() : null))
      .then((data: { type?: string } | null) => {
        if (cancelled || !data) return
        if (data.type && data.type !== "channel") router.replace(`/chat/${channelId}`)
      })
      .catch(() => {})
    return () => { cancelled = true }
  }, [status, session?.accessToken, channelId, router])

  return <RoomView roomId={channelId} surface="channels" />
}
