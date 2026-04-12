"use client"

import { useEffect, useState } from "react"
import Avatar from "@/components/ui/Avatar"
import Button from "@/components/ui/Button"
import ConfirmDialog from "@/components/ui/ConfirmDialog"
import Input from "@/components/ui/Input"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface MemberProfile {
  user_id: string
  username: string
  display_name: string
  avatar_url: string
  is_admin: boolean
  joined_at: string
}

interface Contact {
  user_id: string
  username: string
  email: string
  display_name: string
  avatar_url: string
}

interface Props {
  roomId: string
  roomName: string
  roomType?: "group" | "channel"
  currentUserId: string
  token: string
  onClose: () => void
  onNameUpdated: (name: string) => void
  onLeft: () => void
}

export default function GroupMembersPanel({
  roomId,
  roomName,
  roomType = "group",
  currentUserId,
  token,
  onClose,
  onNameUpdated,
  onLeft,
}: Props) {
  const [members, setMembers] = useState<MemberProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  const [renaming, setRenaming] = useState(false)
  const [newName, setNewName] = useState(roomName)
  const [renameLoading, setRenameLoading] = useState(false)

  const [addingMember, setAddingMember] = useState(false)
  const [contacts, setContacts] = useState<Contact[]>([])
  const [contactsLoading, setContactsLoading] = useState(false)
  const [addLoading, setAddLoading] = useState<string | null>(null)

  const [removeConfirm, setRemoveConfirm] = useState<MemberProfile | null>(null)
  const [removeLoading, setRemoveLoading] = useState(false)

  const isAdmin = members.some((m) => m.user_id === currentUserId && m.is_admin)
  const memberIds = new Set(members.map((m) => m.user_id))

  function loadMembers() {
    setLoading(true)
    setError("")
    fetch(`${API_URL}/chat/rooms/${roomId}/members`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((r) => r.json())
      .then((data: MemberProfile[]) => setMembers(data))
      .catch(() => setError("Failed to load members."))
      .finally(() => setLoading(false))
  }

  useEffect(() => { loadMembers() }, [roomId, token]) // eslint-disable-line react-hooks/exhaustive-deps

  async function handleRename() {
    if (!newName.trim()) return
    setRenameLoading(true)
    try {
      const res = await fetch(`${API_URL}/chat/rooms/${roomId}`, {
        method: "PUT",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({ name: newName.trim() }),
      })
      if (res.ok) {
        onNameUpdated(newName.trim())
        setRenaming(false)
      } else {
        const data = await res.json().catch(() => ({}))
        setError((data as { error?: string }).error ?? "Failed to rename group.")
      }
    } catch {
      setError("Network error. Please try again.")
    } finally {
      setRenameLoading(false)
    }
  }

  async function loadContacts() {
    setContactsLoading(true)
    try {
      const res = await fetch(`${API_URL}/contacts`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      const data: Contact[] = await res.json()
      setContacts(data.filter((c) => !memberIds.has(c.user_id)))
    } catch {
      setError("Failed to load contacts.")
    } finally {
      setContactsLoading(false)
    }
  }

  async function handleAdd(userId: string) {
    setAddLoading(userId)
    try {
      const res = await fetch(`${API_URL}/chat/rooms/${roomId}/members`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({ user_id: userId }),
      })
      if (res.ok) {
        setAddingMember(false)
        loadMembers()
      } else {
        const data = await res.json().catch(() => ({}))
        setError((data as { error?: string }).error ?? "Failed to add member.")
      }
    } catch {
      setError("Network error. Please try again.")
    } finally {
      setAddLoading(null)
    }
  }

  async function handleRemove() {
    if (!removeConfirm) return
    const isSelf = removeConfirm.user_id === currentUserId

    // Channels are ephemeral — leaving is just navigating away (WS disconnects automatically).
    if (roomType === "channel" && isSelf) {
      setRemoveConfirm(null)
      onLeft()
      return
    }

    setRemoveLoading(true)
    try {
      const res = await fetch(
        `${API_URL}/chat/rooms/${roomId}/members/${removeConfirm.user_id}`,
        { method: "DELETE", headers: { Authorization: `Bearer ${token}` } }
      )
      if (res.ok) {
        setRemoveConfirm(null)
        if (isSelf) {
          onLeft()
        } else {
          loadMembers()
        }
      } else {
        const data = await res.json().catch(() => ({}))
        setError((data as { error?: string }).error ?? "Failed to remove member.")
      }
    } catch {
      setError("Network error. Please try again.")
    } finally {
      setRemoveLoading(false)
    }
  }

  const isSelfLeave = removeConfirm?.user_id === currentUserId

  return (
    <div className="flex h-full flex-col bg-gray-900">
      <ConfirmDialog
        open={!!removeConfirm}
        title={isSelfLeave ? (roomType === "channel" ? "Leave channel" : "Leave group") : "Remove member"}
        message={
          isSelfLeave
            ? `Leave this ${roomType === "channel" ? "channel" : "group"}? You will no longer receive messages.`
            : `Remove ${removeConfirm?.display_name || removeConfirm?.username} from the ${roomType === "channel" ? "channel" : "group"}?`
        }
        confirmLabel={isSelfLeave ? "Leave" : "Remove"}
        loading={removeLoading}
        onConfirm={handleRemove}
        onCancel={() => setRemoveConfirm(null)}
      />

      {/* Header */}
      <div className="flex items-center gap-3 border-b border-gray-800 px-4 py-3">
        <button
          type="button"
          aria-label="Close panel"
          onClick={onClose}
          className="text-gray-400 hover:text-gray-200"
        >
          ←
        </button>
        <h2 className="flex-1 text-sm font-semibold text-white">
          {roomType === "channel" ? "Channel members" : "Group members"}
        </h2>
      </div>

      <div className="flex-1 overflow-y-auto p-4 space-y-5">
        {/* Name display / rename (groups only) */}
        <div>
          {roomType === "group" && renaming ? (
            <div className="flex items-end gap-2">
              <div className="flex-1">
                <Input
                  label="Group name"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  autoFocus
                />
              </div>
              <Button
                variant="primary"
                size="sm"
                onClick={handleRename}
                loading={renameLoading}
                disabled={!newName.trim()}
              >
                Save
              </Button>
              <Button variant="ghost" size="sm" onClick={() => { setRenaming(false); setNewName(roomName) }}>
                Cancel
              </Button>
            </div>
          ) : (
            <div className="flex items-center gap-2">
              <p className="text-sm font-semibold text-white">
                {roomType === "channel" ? "# " : ""}{roomName}
              </p>
              {roomType === "group" && isAdmin && (
                <button
                  type="button"
                  onClick={() => { setNewName(roomName); setRenaming(true) }}
                  className="text-xs text-indigo-400 hover:text-indigo-300"
                  aria-label="Rename group"
                >
                  Edit
                </button>
              )}
            </div>
          )}
        </div>

        {/* Error */}
        {error && <p className="text-xs text-red-400">{error}</p>}

        {/* Member list */}
        {loading ? (
          <p className="text-xs text-gray-500">Loading…</p>
        ) : (
          <ul className="divide-y divide-gray-800 rounded-lg ring-1 ring-gray-800">
            {members.map((m) => (
              <li key={m.user_id} className="flex items-center gap-3 px-4 py-3">
                <Avatar src={m.avatar_url} name={m.display_name || m.username || "?"} size="sm" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm text-white">{m.display_name || m.username}</p>
                  {m.is_admin && (
                    <p className="text-xs text-indigo-400">Admin</p>
                  )}
                </div>
                {/* Groups only: admin removes non-admin others */}
                {roomType === "group" && isAdmin && m.user_id !== currentUserId && !m.is_admin && (
                  <button
                    type="button"
                    onClick={() => setRemoveConfirm(m)}
                    className="shrink-0 text-xs text-gray-500 hover:text-red-400"
                  >
                    Remove
                  </button>
                )}
                {/* Self-leave: channels allow anyone; groups only allow non-admins */}
                {m.user_id === currentUserId && (roomType === "channel" || !m.is_admin) && (
                  <button
                    type="button"
                    onClick={() => setRemoveConfirm(m)}
                    className="shrink-0 text-xs text-gray-500 hover:text-red-400"
                  >
                    Leave
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}

        {/* Add member (groups only, admin only) */}
        {roomType === "group" && isAdmin && (
          <div>
            {addingMember ? (
              <div className="space-y-2">
                <p className="text-xs font-medium text-gray-400">Select a contact to add</p>
                {contactsLoading ? (
                  <p className="text-xs text-gray-500">Loading…</p>
                ) : contacts.length === 0 ? (
                  <p className="text-xs text-gray-500">No contacts available to add.</p>
                ) : (
                  <ul className="max-h-48 overflow-y-auto divide-y divide-gray-800 rounded-lg ring-1 ring-gray-800">
                    {contacts.map((c) => (
                      <li key={c.user_id} className="flex items-center gap-3 px-4 py-3">
                        <Avatar src={c.avatar_url} name={c.display_name || c.username || "?"} size="sm" />
                        <span className="flex-1 truncate text-sm text-white">
                          {c.display_name || c.username}
                        </span>
                        <Button
                          variant="secondary"
                          size="sm"
                          loading={addLoading === c.user_id}
                          onClick={() => handleAdd(c.user_id)}
                        >
                          Add
                        </Button>
                      </li>
                    ))}
                  </ul>
                )}
                <Button variant="ghost" size="sm" onClick={() => setAddingMember(false)}>
                  Cancel
                </Button>
              </div>
            ) : (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => { setAddingMember(true); loadContacts() }}
              >
                + Add member
              </Button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
