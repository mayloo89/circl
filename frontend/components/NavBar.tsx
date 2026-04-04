"use client"

import Link from "next/link"
import { useSession } from "next-auth/react"
import { useEffect, useState } from "react"

import { useNotificationsContext } from "@/contexts/NotificationsContext"
import { usePush } from "@/hooks/usePush"
import Avatar from "@/components/ui/Avatar"
import Badge from "@/components/ui/Badge"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

function BellIcon({ muted }: { muted?: boolean }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {muted ? (
        <>
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
          <path d="M18.63 13A17.9 17.9 0 0 1 18 8" />
          <path d="M6.26 6.26A5.86 5.86 0 0 0 6 8c0 7-3 9-3 9h14" />
          <path d="M18 8a6 6 0 0 0-9.33-5" />
          <line x1="1" y1="1" x2="23" y2="23" />
        </>
      ) : (
        <>
          <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
        </>
      )}
    </svg>
  )
}

export default function NavBar() {
  const { data: session, status } = useSession()
  const { pendingCount, unreadChatCount } = useNotificationsContext()
  const [avatarURL, setAvatarURL] = useState("")
  const [displayName, setDisplayName] = useState("")
  const { permission, supported, enable, disable } = usePush(session?.accessToken)

  useEffect(() => {
    if (status !== "authenticated" || !session?.accessToken) return

    fetch(`${API_URL}/profiles/me`, {
      headers: { Authorization: `Bearer ${session.accessToken}` },
    })
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (data) {
          setAvatarURL(data.avatar_url ?? "")
          setDisplayName(data.display_name ?? "")
        }
      })
      .catch(() => {})
  }, [status, session])

  if (status !== "authenticated") return null

  return (
    <nav className="flex items-center bg-gray-900 px-6 py-3 shadow ring-1 ring-gray-800">
      <Link href="/" className="text-lg font-bold text-white hover:text-gray-300">Circl</Link>
      <div className="ml-auto flex items-center gap-6">
        <Link href="/browse" className="text-sm text-gray-300 hover:text-white">Browse</Link>
        <Link href="/chat" className="relative text-sm text-gray-300 hover:text-white">
          Messages
          {unreadChatCount > 0 && (
            <Badge count={unreadChatCount} max={9} variant="dot" className="absolute -right-4 -top-2" />
          )}
        </Link>
        <Link href="/chat/channels" className="text-sm text-gray-300 hover:text-white">Channels</Link>
        <Link href="/contacts" className="relative text-sm text-gray-300 hover:text-white">
          Contacts
          {pendingCount > 0 && (
            <Badge count={pendingCount} max={9} variant="dot" className="absolute -right-4 -top-2" />
          )}
        </Link>
        {supported && permission !== "granted" && (
          <button
            onClick={enable}
            title="Enable push notifications"
            className="text-gray-400 hover:text-white"
            aria-label="Enable push notifications"
          >
            <BellIcon muted />
          </button>
        )}
        {supported && permission === "granted" && (
          <button
            onClick={disable}
            title="Disable push notifications"
            className="text-green-400 hover:text-gray-400"
            aria-label="Disable push notifications"
          >
            <BellIcon />
          </button>
        )}
        <Link href="/profile" className="flex items-center gap-2 text-sm text-gray-300 hover:text-white">
          <Avatar src={avatarURL} name={displayName || "?"} size="xs" />
          Profile
        </Link>
      </div>
    </nav>
  )
}
