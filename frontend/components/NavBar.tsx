"use client"

import Link from "next/link"
import { useSession } from "next-auth/react"
import { useEffect, useState } from "react"

import { useNotificationsContext } from "@/contexts/NotificationsContext"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export default function NavBar() {
  const { data: session, status } = useSession()
  const { pendingCount, unreadChatCount } = useNotificationsContext()
  const [avatarURL, setAvatarURL] = useState("")
  const [displayName, setDisplayName] = useState("")

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
        <Link href="/chat" className="relative text-sm text-gray-300 hover:text-white">
          Messages
          {unreadChatCount > 0 && (
            <span className="absolute -right-4 -top-2 flex h-4 w-4 items-center justify-center rounded-full bg-indigo-600 text-[10px] font-semibold text-white">
              {unreadChatCount > 9 ? "9+" : unreadChatCount}
            </span>
          )}
        </Link>
        <Link href="/contacts" className="relative text-sm text-gray-300 hover:text-white">
          Contacts
          {pendingCount > 0 && (
            <span className="absolute -right-4 -top-2 flex h-4 w-4 items-center justify-center rounded-full bg-indigo-600 text-[10px] font-semibold text-white">
              {pendingCount > 9 ? "9+" : pendingCount}
            </span>
          )}
        </Link>
        <Link href="/profile" className="flex items-center gap-2 text-sm text-gray-300 hover:text-white">
          {avatarURL ? (
            <img src={avatarURL} alt="" className="h-6 w-6 rounded-full object-cover ring-1 ring-gray-700" />
          ) : (
            <span className="flex h-6 w-6 items-center justify-center rounded-full bg-gray-700 text-xs text-gray-300 ring-1 ring-gray-600">
              {displayName ? displayName[0].toUpperCase() : "?"}
            </span>
          )}
          Profile
        </Link>
      </div>
    </nav>
  )
}
