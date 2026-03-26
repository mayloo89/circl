"use client"

import Image from "next/image"
import { useSession } from "next-auth/react"
import { useRouter } from "next/navigation"
import { useCallback, useEffect, useRef, useState } from "react"

import { useNotificationsContext } from "@/contexts/NotificationsContext"
import { usePresence } from "@/hooks/usePresence"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface UserSummary {
  id: string
  email: string
  display_name: string
  avatar_url: string
}

interface PendingRequest {
  contact_id: string
  user_id: string
  email: string
  display_name: string
  avatar_url: string
}

interface SentRequest {
  contact_id: string
  user_id: string
  email: string
  display_name: string
  avatar_url: string
}

interface AcceptedContact {
  contact_id: string
  user_id: string
  email: string
  display_name: string
  avatar_url: string
}

export default function ContactsPage() {
  const { data: session, status } = useSession()
  const router = useRouter()

  const [contacts, setContacts] = useState<AcceptedContact[]>([])
  const [pending, setPending] = useState<PendingRequest[]>([])
  const [sent, setSent] = useState<SentRequest[]>([])
  const [searchQuery, setSearchQuery] = useState("")
  const [searchResults, setSearchResults] = useState<UserSummary[]>([])
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(true)

  const token = session?.accessToken
  const { subscribe, refreshPendingCount } = useNotificationsContext()
  const contactUserIDs = contacts.map((c) => c.user_id)
  const presence = usePresence(contactUserIDs, token, subscribe)

  useEffect(() => {
    if (status === "unauthenticated") router.push("/login")
  }, [status, router])

  const fetchPending = useCallback(async () => {
    if (!token) return
    try {
      const res = await fetch(`${API_URL}/contacts/pending`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (res.ok) setPending(await res.json())
    } catch {
      // silent — non-critical refresh
    }
  }, [token])

  useEffect(() => {
    if (status !== "authenticated" || !token) return

    Promise.all([
      fetch(`${API_URL}/contacts`, { headers: { Authorization: `Bearer ${token}` } }).then((r) => r.ok ? r.json() : []),
      fetch(`${API_URL}/contacts/pending`, { headers: { Authorization: `Bearer ${token}` } }).then((r) => r.ok ? r.json() : []),
      fetch(`${API_URL}/contacts/sent`, { headers: { Authorization: `Bearer ${token}` } }).then((r) => r.ok ? r.json() : []),
    ])
      .then(([c, p, s]) => {
        setContacts(c)
        setPending(p)
        setSent(s)
      })
      .catch(() => setError("Failed to load contacts."))
      .finally(() => setLoading(false))
  }, [status, token])

  // Keep a ref to the event handler so the subscription (registered once on
  // mount) always calls the latest version — avoids stale closures over
  // `sent`, `pending`, etc. without re-subscribing on every state change.
  const handleEventRef = useRef<Parameters<typeof subscribe>[0]>(() => {})
  useEffect(() => {
    handleEventRef.current = (e) => {
      if (e.type === "contact_request") {
        // Re-fetch to get the requester's display_name / email.
        fetchPending()
      }
      if (e.type === "contact_accepted") {
        // Move the matching sent entry into accepted contacts.
        // Read `sent` outside the updater to avoid nested setState calls
        // (double-invocation in StrictMode).
        const matched = sent.find((s) => s.contact_id === e.payload.contact_id)
        if (matched) {
          setSent((prev) => prev.filter((s) => s.contact_id !== e.payload.contact_id))
          setContacts((prev) => {
            if (prev.some((c) => c.contact_id === matched.contact_id)) return prev
            return [...prev, {
              contact_id: matched.contact_id,
              user_id: matched.user_id,
              email: matched.email,
              display_name: matched.display_name,
              avatar_url: matched.avatar_url,
            }]
          })
        }
      }
      if (e.type === "contact_removed") {
        const { contact_id } = e.payload
        setContacts((prev) => prev.filter((c) => c.contact_id !== contact_id))
        setPending((prev) => prev.filter((r) => r.contact_id !== contact_id))
        setSent((prev) => prev.filter((r) => r.contact_id !== contact_id))
      }
    }
  })

  // Stable subscription: registers once on mount, calls through the ref.
  useEffect(() => {
    return subscribe((e) => handleEventRef.current(e))
  }, [subscribe])

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
    const contact = await res.json()
    const user = searchResults.find((u) => u.id === addresseeID)
    if (user) {
      setSent((prev) => [...prev, { contact_id: contact.id, user_id: user.id, email: user.email, display_name: user.display_name, avatar_url: user.avatar_url }])
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
    const accepted = pending.find((r) => r.contact_id === contactID)
    setPending((prev) => prev.filter((r) => r.contact_id !== contactID))
    if (accepted) setContacts((prev) => [...prev, { contact_id: contactID, user_id: accepted.user_id, email: accepted.email, display_name: accepted.display_name, avatar_url: accepted.avatar_url }])
    refreshPendingCount()
  }

  async function cancelSent(contactID: string) {
    setError("")
    const res = await fetch(`${API_URL}/contacts/${contactID}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!res.ok) {
      setError("Failed to cancel request.")
      return
    }
    setSent((prev) => prev.filter((r) => r.contact_id !== contactID))
  }

  async function startDM(peerID: string) {
    setError("")
    const res = await fetch(`${API_URL}/chat/rooms/dm`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ peer_id: peerID }),
    })
    if (!res.ok) {
      setError("Failed to open conversation.")
      return
    }
    const room = await res.json()
    router.push(`/chat/${room.id}`)
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
    setContacts((prev) => prev.filter((c) => c.contact_id !== contactID))
    setPending((prev) => prev.filter((r) => r.contact_id !== contactID))
    refreshPendingCount()
  }

  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-950">
        <p className="text-gray-400">Loading...</p>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
      <div className="w-full max-w-lg space-y-8 px-4">
        <div className="flex items-center justify-between">
          <h1 className="text-3xl font-bold text-white">Contacts</h1>
          <button aria-label="Go to home" onClick={() => router.push("/")} className="text-sm text-gray-400 hover:text-gray-200">
            ← Home
          </button>
        </div>

        {error && (
          <p className="rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">{error}</p>
        )}

        {/* Search */}
        <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
          <h2 className="mb-3 text-lg font-semibold text-white">Add Contact</h2>
          <label htmlFor="contact-search" className="sr-only">Search contacts</label>
          <input
            id="contact-search"
            type="text"
            placeholder="Search by name or email..."
            value={searchQuery}
            onChange={(e) => search(e.target.value)}
            className="w-full rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          />
          {searchResults.length > 0 && (
            <ul className="mt-3 divide-y divide-gray-700">
              {searchResults.map((u) => (
                <li key={u.id} className="flex items-center justify-between py-2">
                  <div className="flex items-center gap-3">
                    {u.avatar_url ? (
                      <Image src={u.avatar_url} alt="" width={32} height={32} className="h-8 w-8 flex-none rounded-full object-cover ring-1 ring-gray-700" />
                    ) : (
                      <span className="flex h-8 w-8 flex-none items-center justify-center rounded-full bg-gray-700 text-sm text-gray-300 ring-1 ring-gray-600">
                        {(u.display_name || u.email)[0].toUpperCase()}
                      </span>
                    )}
                    <span className="text-sm text-gray-200">{u.display_name || u.email}</span>
                  </div>
                  <button
                    onClick={() => sendRequest(u.id)}
                    className="rounded bg-indigo-600 px-3 py-2 text-xs text-white hover:bg-indigo-500"
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
          <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
            <h2 className="mb-3 text-lg font-semibold text-white">
              Pending Requests
              <span className="ml-2 rounded-full bg-indigo-600 px-2 py-0.5 text-xs text-white">
                {pending.length}
              </span>
            </h2>
            <ul className="divide-y divide-gray-700">
              {pending.map((r) => (
                <li key={r.contact_id} className="flex items-center justify-between py-2">
                  <div className="flex items-center gap-3">
                    {r.avatar_url ? (
                      <Image src={r.avatar_url} alt="" width={32} height={32} className="h-8 w-8 flex-none rounded-full object-cover ring-1 ring-gray-700" />
                    ) : (
                      <span className="flex h-8 w-8 flex-none items-center justify-center rounded-full bg-gray-700 text-sm text-gray-300 ring-1 ring-gray-600">
                        {(r.display_name || r.email)[0].toUpperCase()}
                      </span>
                    )}
                    <span className="text-sm text-gray-200">{r.display_name || r.email}</span>
                  </div>
                  <div className="flex gap-2">
                    <button
                      onClick={() => accept(r.contact_id)}
                      className="rounded bg-green-700 px-3 py-2 text-xs text-white hover:bg-green-600"
                    >
                      Accept
                    </button>
                    <button
                      onClick={() => remove(r.contact_id)}
                      className="rounded bg-gray-700 px-3 py-2 text-xs text-gray-200 hover:bg-gray-600"
                    >
                      Decline
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Sent requests */}
        {sent.length > 0 && (
          <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
            <h2 className="mb-3 text-lg font-semibold text-white">Sent Requests</h2>
            <ul className="divide-y divide-gray-700">
              {sent.map((r) => (
                <li key={r.contact_id} className="flex items-center justify-between py-2">
                  <div className="flex items-center gap-3">
                    {r.avatar_url ? (
                      <Image src={r.avatar_url} alt="" width={32} height={32} className="h-8 w-8 flex-none rounded-full object-cover ring-1 ring-gray-700" />
                    ) : (
                      <span className="flex h-8 w-8 flex-none items-center justify-center rounded-full bg-gray-700 text-sm text-gray-300 ring-1 ring-gray-600">
                        {(r.display_name || r.email)[0].toUpperCase()}
                      </span>
                    )}
                    <span className="text-sm text-gray-200">{r.display_name || r.email}</span>
                  </div>
                  <button
                    onClick={() => cancelSent(r.contact_id)}
                    className="rounded bg-gray-700 px-3 py-2 text-xs text-gray-200 hover:bg-gray-600"
                  >
                    Cancel
                  </button>
                </li>
              ))}
            </ul>
          </div>
        )}

        {/* Contact list */}
        <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
          <h2 className="mb-3 text-lg font-semibold text-white">My Contacts ({contacts.length})</h2>
          {contacts.length === 0 ? (
            <p className="text-sm text-gray-500">No contacts yet. Use the search above to add someone.</p>
          ) : (
            <ul className="divide-y divide-gray-700">
              {contacts.map((c) => (
                <li key={c.contact_id} className="flex items-center justify-between py-2">
                  <div className="flex items-center gap-3">
                    <div className="relative flex-none">
                      {c.avatar_url ? (
                        <Image src={c.avatar_url} alt="" width={32} height={32} className="h-8 w-8 rounded-full object-cover ring-1 ring-gray-700" />
                      ) : (
                        <span className="flex h-8 w-8 items-center justify-center rounded-full bg-gray-700 text-sm text-gray-300 ring-1 ring-gray-600">
                          {(c.display_name || c.email)[0].toUpperCase()}
                        </span>
                      )}
                      <span
                        className={`absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full ring-2 ring-gray-900 ${
                          presence[c.user_id]?.online ? "bg-green-400" : "bg-gray-600"
                        }`}
                      />
                    </div>
                    <span className="text-sm text-gray-200">{c.display_name || c.email}</span>
                  </div>
                  <div className="flex gap-2">
                    <button
                      onClick={() => startDM(c.user_id)}
                      className="rounded bg-indigo-700 px-3 py-2 text-xs text-white hover:bg-indigo-600"
                    >
                      Message
                    </button>
                    <button
                      onClick={() => remove(c.contact_id)}
                      className="rounded bg-red-900 px-3 py-2 text-xs text-red-300 hover:bg-red-800"
                    >
                      Remove
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  )
}
