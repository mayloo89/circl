"use client"

import Link from "next/link"
import { signOut, useSession } from "next-auth/react"
import { useEffect, useRef, useState } from "react"

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
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
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

  useEffect(() => {
    if (!menuOpen) return
    function handleClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false)
      }
    }
    document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [menuOpen])

  async function handleSignOut() {
    const token = session?.accessToken
    if (token) {
      try {
        await fetch(`${API_URL}/presence/heartbeat`, {
          method: "DELETE",
          headers: { Authorization: `Bearer ${token}` },
        })
      } catch {
        // non-critical
      }
    }
    await signOut()
  }

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

        {/* Avatar + dropdown */}
        <div ref={menuRef} className="relative">
          <button
            onClick={() => setMenuOpen((v) => !v)}
            className="rounded-full focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-900"
            aria-label="Open user menu"
            aria-haspopup="true"
            aria-expanded={menuOpen}
          >
            <Avatar src={avatarURL} name={displayName || "?"} size="xs" />
          </button>

          {menuOpen && (
            <div
              role="menu"
              className="absolute right-0 top-full z-50 mt-2 w-44 rounded-lg bg-gray-800 py-1 shadow-lg ring-1 ring-gray-700"
            >
              <Link
                href="/profile"
                role="menuitem"
                onClick={() => setMenuOpen(false)}
                className="block px-4 py-2 text-sm text-gray-200 hover:bg-gray-700 hover:text-white"
              >
                Profile
              </Link>
              <Link
                href="/settings"
                role="menuitem"
                onClick={() => setMenuOpen(false)}
                className="block px-4 py-2 text-sm text-gray-200 hover:bg-gray-700 hover:text-white"
              >
                Settings
              </Link>
              <div className="my-1 border-t border-gray-700" />
              <button
                role="menuitem"
                onClick={handleSignOut}
                className="block w-full px-4 py-2 text-left text-sm text-red-400 hover:bg-gray-700 hover:text-red-300"
              >
                Log out
              </button>
            </div>
          )}
        </div>
      </div>
    </nav>
  )
}
