"use client"

import { useSession } from "next-auth/react"
import { useRouter } from "next/navigation"
import { useEffect, useState } from "react"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface UserSummary {
  id: string
  email: string
  display_name: string
}

export default function ContactsPage() {
  const { data: session, status } = useSession()
  const router = useRouter()

  const [contacts, setContacts] = useState<UserSummary[]>([])
  const [pending, setPending] = useState<UserSummary[]>([])
  const [searchQuery, setSearchQuery] = useState("")
  const [searchResults, setSearchResults] = useState<UserSummary[]>([])
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(true)

  const token = session?.accessToken

  useEffect(() => {
    if (status === "unauthenticated") router.push("/login")
  }, [status, router])

  useEffect(() => {
    if (status !== "authenticated" || !token) return

    Promise.all([
      fetch(`${API_URL}/contacts`, { headers: { Authorization: `Bearer ${token}` } }).then((r) => r.json()),
      fetch(`${API_URL}/contacts/pending`, { headers: { Authorization: `Bearer ${token}` } }).then((r) => r.json()),
    ])
      .then(([c, p]) => {
        setContacts(c)
        setPending(p)
      })
      .catch(() => setError("Failed to load contacts."))
      .finally(() => setLoading(false))
  }, [status, token])

  async function search(q: string) {
    setSearchQuery(q)
    if (!q.trim()) {
      setSearchResults([])
      return
    }
    try {
      const res = await fetch(`${API_URL}/users/search?q=${encodeURIComponent(q)}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      const data = await res.json()
      setSearchResults(data)
    } catch {
      setSearchResults([])
    }
  }

  async function sendRequest(addresseeID: string) {
    setError("")
    const res = await fetch(`${API_URL}/contacts`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ addressee_id: addresseeID }),
    })
    if (res.status === 409) {
      setError("Contact request already sent.")
      return
    }
    if (!res.ok) {
      setError("Failed to send contact request.")
      return
    }
    setSearchResults((prev) => prev.filter((u) => u.id !== addresseeID))
  }

  async function accept(contactID: string) {
    setError("")
    const res = await fetch(`${API_URL}/contacts/${contactID}/accept`, {
      method: "PUT",
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!res.ok) {
      setError("Failed to accept contact.")
      return
    }
    const accepted = pending.find((u) => u.id === contactID)
    setPending((prev) => prev.filter((u) => u.id !== contactID))
    if (accepted) setContacts((prev) => [...prev, accepted])
  }

  async function remove(contactID: string) {
    setError("")
    const res = await fetch(`${API_URL}/contacts/${contactID}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!res.ok) {
      setError("Failed to remove contact.")
      return
    }
    setContacts((prev) => prev.filter((u) => u.id !== contactID))
    setPending((prev) => prev.filter((u) => u.id !== contactID))
  }

  function displayName(u: UserSummary) {
    return u.display_name || u.email
  }

  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-gray-500">Loading...</p>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen flex-col items-center bg-gray-50 py-10">
      <div className="w-full max-w-lg space-y-8 px-4">
        <div className="flex items-center justify-between">
          <h1 className="text-3xl font-bold">Contacts</h1>
          <button onClick={() => router.push("/")} className="text-sm text-gray-500 hover:text-gray-700">
            ← Home
          </button>
        </div>

        {error && <p className="rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</p>}

        {/* Search */}
        <div className="rounded-lg bg-white p-6 shadow">
          <h2 className="mb-3 text-lg font-semibold">Add Contact</h2>
          <input
            type="text"
            placeholder="Search by name or email..."
            value={searchQuery}
            onChange={(e) => search(e.target.value)}
            className="w-full rounded-md border border-gray-300 px-3 py-2 shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          />
          {searchResults.length > 0 && (
            <ul className="mt-3 divide-y divide-gray-100">
              {searchResults.map((u) => (
                <li key={u.id} className="flex items-center justify-between py-2">
                  <span className="text-sm">{displayName(u)}</span>
                  <button
                    onClick={() => sendRequest(u.id)}
                    className="rounded bg-indigo-600 px-3 py-1 text-xs text-white hover:bg-indigo-700"
                  >
                    Add
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* Pending requests */}
        {pending.length > 0 && (
          <div className="rounded-lg bg-white p-6 shadow">
            <h2 className="mb-3 text-lg font-semibold">Pending Requests</h2>
            <ul className="divide-y divide-gray-100">
              {pending.map((u) => (
                <li key={u.id} className="flex items-center justify-between py-2">
                  <span className="text-sm">{displayName(u)}</span>
                  <div className="flex gap-2">
                    <button
                      onClick={() => accept(u.id)}
                      className="rounded bg-green-600 px-3 py-1 text-xs text-white hover:bg-green-700"
                    >
                      Accept
                    </button>
                    <button
                      onClick={() => remove(u.id)}
                      className="rounded bg-gray-200 px-3 py-1 text-xs text-gray-700 hover:bg-gray-300"
                    >
                      Decline
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Contact list */}
        <div className="rounded-lg bg-white p-6 shadow">
          <h2 className="mb-3 text-lg font-semibold">My Contacts ({contacts.length})</h2>
          {contacts.length === 0 ? (
            <p className="text-sm text-gray-500">No contacts yet. Use the search above to add someone.</p>
          ) : (
            <ul className="divide-y divide-gray-100">
              {contacts.map((u) => (
                <li key={u.id} className="flex items-center justify-between py-2">
                  <span className="text-sm">{displayName(u)}</span>
                  <button
                    onClick={() => remove(u.id)}
                    className="rounded bg-red-100 px-3 py-1 text-xs text-red-700 hover:bg-red-200"
                  >
                    Remove
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  )
}
