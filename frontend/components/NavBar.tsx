"use client"

import Link from "next/link"
import { useSession } from "next-auth/react"

import { useNotificationsContext } from "@/contexts/NotificationsContext"

export default function NavBar() {
  const { status } = useSession()
  const { pendingCount } = useNotificationsContext()

  if (status !== "authenticated") return null

  return (
    <nav className="flex items-center bg-gray-900 px-6 py-3 shadow ring-1 ring-gray-800">
      <Link href="/" className="text-lg font-bold text-white hover:text-gray-300">Circl</Link>
      <div className="ml-auto flex items-center gap-6">
        <Link href="/chat" className="text-sm text-gray-300 hover:text-white">
          Messages
        </Link>
        <Link href="/contacts" className="relative text-sm text-gray-300 hover:text-white">
          Contacts
          {pendingCount > 0 && (
            <span className="absolute -right-4 -top-2 flex h-4 w-4 items-center justify-center rounded-full bg-indigo-600 text-[10px] font-semibold text-white">
              {pendingCount > 9 ? "9+" : pendingCount}
            </span>
          )}
        </Link>
        <Link href="/profile" className="text-sm text-gray-300 hover:text-white">
          Profile
        </Link>
      </div>
    </nav>
  )
}
