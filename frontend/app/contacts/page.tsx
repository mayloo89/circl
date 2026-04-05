"use client"

import { useSession } from "next-auth/react"
import { useRouter } from "next/navigation"
import { useCallback, useEffect, useRef, useState } from "react"

import { useNotificationsContext } from "@/contexts/NotificationsContext"
import { usePresence } from "@/hooks/usePresence"
import Avatar from "@/components/ui/Avatar"
import Badge from "@/components/ui/Badge"
import Button from "@/components/ui/Button"
import ConfirmDialog from "@/components/ui/ConfirmDialog"
import ContactCard from "@/components/contacts/ContactCard"
import SearchBar from "@/components/contacts/SearchBar"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface UserSummary {
  id: string
  username: string
  email: string
  display_name: string
  avatar_url: string
}

interface PendingRequest {
  contact_id: string
  user_id: string
  username: string
  email: string
  display_name: string
  avatar_url: string
}

interface SentRequest {
  contact_id: string
  user_id: string
  username: string
  email: string
  display_name: string
  avatar_url: string
}

interface AcceptedContact {
  contact_id: string
  user_id: string
  username: string
  email: string
  display_name: string
  avatar_url: string
}

interface BlockedUser {
  block_id: string
  user_id: string
  username: string
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
  const [blocked, setBlocked] = useState<BlockedUser[]>([])
  const [searchQuery, setSearchQuery] = useState("")
  const [searchResults, setSearchResults] = useState<UserSummary[]>([])
  const [removeConfirm, setRemoveConfirm] = useState<AcceptedContact | null>(null)
  const [unblockConfirm, setUnblockConfirm] = useState<BlockedUser | null>(null)
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(true)

  const token = session?.accessToken
  const { subscribe, refreshPendingCount } = useNotificationsContext()
  const contactUserIDs = contacts.map((c) => c.user_id)
  const presence = usePresence(contactUserIDs, token, subscribe)

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
      fetch(`${API_URL}/contacts/blocked`, { headers: { Authorization: `Bearer ${token}` } }).then((r) => r.ok ? r.json() : []),
    ])
      .then(([c, p, s, bl]) => { setContacts(c); setPending(p); setSent(s); setBlocked(bl) })
      .catch(() => setError("Failed to load contacts."))
      .finally(() => setLoading(false))
  }, [status, token])

  // Stable subscription via ref to avoid stale closures.
  const handleEventRef = useRef<Parameters<typeof subscribe>[0]>(() => {})
  useEffect(() => {
    handleEventRef.current = (e) => {
      if (e.type === "contact_request") { fetchPending() }
      if (e.type === "contact_accepted") {
        const matched = sent.find((s) => s.contact_id === e.payload.contact_id)
        if (matched) {
          setSent((prev) => prev.filter((s) => s.contact_id !== e.payload.contact_id))
          setContacts((prev) => {
            if (prev.some((c) => c.contact_id === matched.contact_id)) return prev
            return [...prev, { contact_id: matched.contact_id, user_id: matched.user_id, username: matched.username, email: matched.email, display_name: matched.display_name, avatar_url: matched.avatar_url }]
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
  useEffect(() => { return subscribe((e) => handleEventRef.current(e)) }, [subscribe])

  async function search(q: string) {
    setSearchQuery(q)
    if (!q.trim()) { setSearchResults([]); return }
    try {
      const res = await fetch(`${API_URL}/users/search?q=${encodeURIComponent(q)}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      setSearchResults(await res.json())
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
    if (res.status === 409) { setError("Contact request already sent."); return }
    if (!res.ok) { setError("Failed to send contact request."); return }
    const contact = await res.json()
    const user = searchResults.find((u) => u.id === addresseeID)
    if (user) setSent((prev) => [...prev, { contact_id: contact.id, user_id: user.id, username: user.username, email: user.email, display_name: user.display_name, avatar_url: user.avatar_url }])
    setSearchResults((prev) => prev.filter((u) => u.id !== addresseeID))
  }

  async function accept(contactID: string) {
    setError("")
    const res = await fetch(`${API_URL}/contacts/${contactID}/accept`, {
      method: "PUT",
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!res.ok) { setError("Failed to accept contact."); return }
    const accepted = pending.find((r) => r.contact_id === contactID)
    setPending((prev) => prev.filter((r) => r.contact_id !== contactID))
    if (accepted) setContacts((prev) => [...prev, { contact_id: contactID, user_id: accepted.user_id, username: accepted.username, email: accepted.email, display_name: accepted.display_name, avatar_url: accepted.avatar_url }])
    refreshPendingCount()
  }

  async function cancelSent(contactID: string) {
    setError("")
    const res = await fetch(`${API_URL}/contacts/${contactID}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!res.ok) { setError("Failed to cancel request."); return }
    setSent((prev) => prev.filter((r) => r.contact_id !== contactID))
  }

  async function startDM(peerID: string) {
    setError("")
    const res = await fetch(`${API_URL}/chat/rooms/dm`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ peer_id: peerID }),
    })
    if (!res.ok) { setError("Failed to open conversation."); return }
    const room = await res.json()
    router.push(`/chat/${room.id}`)
  }

  async function remove(contactID: string) {
    setError("")
    const res = await fetch(`${API_URL}/contacts/${contactID}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!res.ok) { setError("Failed to remove contact."); return }
    setContacts((prev) => prev.filter((c) => c.contact_id !== contactID))
    setPending((prev) => prev.filter((r) => r.contact_id !== contactID))
    refreshPendingCount()
  }

  async function unblock(userID: string) {
    setError("")
    const res = await fetch(`${API_URL}/contacts/${userID}/block`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!res.ok) { setError("Failed to unblock user."); return }
    setBlocked((prev) => prev.filter((b) => b.user_id !== userID))
    setUnblockConfirm(null)
  }

  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-950">
        <p className="text-gray-400">Loading...</p>
      </div>
    )
  }

  return (
    <>
    <ConfirmDialog
      open={removeConfirm !== null}
      title="Remove contact"
      message={`Remove ${removeConfirm?.display_name || "this contact"}? You can add them again later.`}
      confirmLabel="Remove"
      onConfirm={() => { if (removeConfirm) { remove(removeConfirm.contact_id); setRemoveConfirm(null) } }}
      onCancel={() => setRemoveConfirm(null)}
    />
    <ConfirmDialog
      open={unblockConfirm !== null}
      title="Unblock user"
      message={`Unblock ${unblockConfirm?.display_name || "this user"}? They will be able to contact you again.`}
      confirmLabel="Unblock"
      onConfirm={() => unblockConfirm && unblock(unblockConfirm.user_id)}
      onCancel={() => setUnblockConfirm(null)}
    />
    <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
      <div className="w-full max-w-lg space-y-8 px-4">
        <div className="flex items-center justify-between">
          <h1 className="text-3xl font-bold text-white">Contacts</h1>
          <Button variant="ghost" aria-label="Go to home" onClick={() => router.push("/")}>← Home</Button>
        </div>

        {error && (
          <p className="rounded-md bg-red-950 p-3 text-sm text-red-400 ring-1 ring-red-900">{error}</p>
        )}

        <SearchBar
          value={searchQuery}
          onChange={search}
          results={searchResults}
          onAdd={sendRequest}
          onNavigate={(usernameOrId) => router.push(`/profile/${usernameOrId}`)}
        />

        {pending.length > 0 && (
          <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
            <h2 className="mb-3 flex items-center text-lg font-semibold text-white">
              Pending Requests
              <Badge count={pending.length} variant="pill" className="ml-2" />
            </h2>
            <ul className="divide-y divide-gray-700">
              {pending.map((r) => (
                <ContactCard
                  key={r.contact_id}
                  userId={r.user_id}
                  email={r.email}
                  displayName={r.display_name}
                  avatarUrl={r.avatar_url}
                  variant="pending"
                  onNavigate={() => router.push(`/profile/${r.username || r.user_id}`)}
                  onPrimary={() => accept(r.contact_id)}
                  onSecondary={() => remove(r.contact_id)}
                />
              ))}
            </ul>
          </div>
        )}

        {sent.length > 0 && (
          <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
            <h2 className="mb-3 text-lg font-semibold text-white">Sent Requests</h2>
            <ul className="divide-y divide-gray-700">
              {sent.map((r) => (
                <ContactCard
                  key={r.contact_id}
                  userId={r.user_id}
                  email={r.email}
                  displayName={r.display_name}
                  avatarUrl={r.avatar_url}
                  variant="sent"
                  onNavigate={() => router.push(`/profile/${r.username || r.user_id}`)}
                  onSecondary={() => cancelSent(r.contact_id)}
                />
              ))}
            </ul>
          </div>
        )}

        <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
          <h2 className="mb-3 text-lg font-semibold text-white">My Contacts ({contacts.length})</h2>
          {contacts.length === 0 ? (
            <p className="text-sm text-gray-500">No contacts yet. Use the search above to add someone.</p>
          ) : (
            <ul className="divide-y divide-gray-700">
              {contacts.map((c) => (
                <ContactCard
                  key={c.contact_id}
                  userId={c.user_id}
                  email={c.email}
                  displayName={c.display_name}
                  avatarUrl={c.avatar_url}
                  variant="contact"
                  online={presence[c.user_id]?.online ?? false}
                  onNavigate={() => router.push(`/profile/${c.username || c.user_id}`)}
                  onPrimary={() => startDM(c.user_id)}
                  onSecondary={() => setRemoveConfirm(c)}
                />
              ))}
            </ul>
          )}
        </div>

        {blocked.length > 0 && (
          <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
            <h2 className="mb-3 text-lg font-semibold text-white">Blocked Users</h2>
            <ul className="divide-y divide-gray-700">
              {blocked.map((b) => (
                <li key={b.block_id} className="flex items-center justify-between gap-3 py-3">
                  <button
                    type="button"
                    onClick={() => router.push(`/profile/${b.username || b.user_id}`)}
                    className="flex items-center gap-3 hover:opacity-80"
                  >
                    <Avatar src={b.avatar_url} name={b.display_name || b.email} size="sm" />
                    <span className="text-sm font-medium text-gray-300">{b.display_name || b.email}</span>
                  </button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setUnblockConfirm(b)}
                  >
                    Unblock
                  </Button>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </div>
    </>
  )
}
