"use client"

import { signOut, useSession } from "next-auth/react"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export default function SignOutButton() {
  const { data: session } = useSession()

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

  return (
    <button
      onClick={handleSignOut}
      className="w-full rounded-md bg-red-700 px-4 py-2 text-white hover:bg-red-600 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 focus:ring-offset-gray-900"
    >
      Sign Out
    </button>
  )
}
