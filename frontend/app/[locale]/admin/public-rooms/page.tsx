"use client"

import { useSession } from "next-auth/react"
import { useCallback, useEffect, useState } from "react"
import Button from "@/components/ui/Button"
import Input from "@/components/ui/Input"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface PublicRoom {
  id: string
  name: string
  description: string
  visibility: string
  creator_id: string
  created_at: string
}

function CreateModal({
  token,
  onDone,
  onClose,
}: {
  token: string
  onDone: () => void
  onClose: () => void
}) {
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  async function submit() {
    setLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/admin/public-rooms`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ name, description }),
      })
      if (!res.ok) {
        const text = await res.text()
        setError(res.status === 409 ? "A public room with that name already exists." : text.trim() || "Failed to create public room")
        return
      }
      onDone()
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="w-full max-w-md rounded-lg bg-gray-900 ring-1 ring-gray-700 p-6 space-y-4">
        <h2 className="text-base font-semibold text-foreground">Create public room</h2>
        <p className="text-xs text-gray-500">
          Public rooms are open to unregistered guests (nickname + age attestation). Messages are text-only and auto-deleted after 24 hours.
        </p>
        {error && <p className="text-sm text-red-400">{error}</p>}
        <div>
          <label className="block text-xs text-gray-400 mb-1">Name</label>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. lounge"
          />
        </div>
        <div>
          <label className="block text-xs text-gray-400 mb-1">Description</label>
          <Input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Optional description"
          />
        </div>
        <div className="flex justify-end gap-3 pt-2">
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button variant="primary" loading={loading} disabled={!name.trim()} onClick={submit}>Create</Button>
        </div>
      </div>
    </div>
  )
}

export default function AdminPublicRoomsPage() {
  const { data: session } = useSession()
  const [rooms, setRooms] = useState<PublicRoom[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const [createOpen, setCreateOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<PublicRoom | null>(null)
  const [deleteLoading, setDeleteLoading] = useState(false)
  const [deleteError, setDeleteError] = useState("")

  const fetchRooms = useCallback(async () => {
    if (!session?.accessToken) return
    setLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/admin/public-rooms`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      })
      if (!res.ok) throw new Error(String(res.status))
      const data = await res.json()
      setRooms(Array.isArray(data) ? data : [])
    } catch {
      setError("Failed to load public rooms")
    } finally {
      setLoading(false)
    }
  }, [session])

  useEffect(() => { fetchRooms() }, [fetchRooms])

  async function confirmDelete() {
    if (!deleteTarget || !session?.accessToken) return
    setDeleteLoading(true)
    setDeleteError("")
    try {
      const res = await fetch(`${API_URL}/admin/public-rooms/${deleteTarget.id}`, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${session.accessToken}` },
      })
      if (!res.ok) {
        const text = await res.text()
        setDeleteError(text.trim() || "Failed to delete public room")
        return
      }
      setDeleteTarget(null)
      await fetchRooms()
    } finally {
      setDeleteLoading(false)
    }
  }

  return (
    <div className="p-4 sm:p-6 md:p-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-foreground">Public rooms</h1>
        <Button variant="primary" size="sm" onClick={() => setCreateOpen(true)}>
          Create public room
        </Button>
      </div>

      {error && <p className="mb-4 text-sm text-red-400">{error}</p>}

      <div className="overflow-x-auto rounded-lg ring-1 ring-gray-800">
        <table className="w-full text-sm text-left">
          <thead className="bg-gray-900 text-xs uppercase tracking-wider text-gray-500">
            <tr>
              <th className="px-4 py-3">Name</th>
              <th className="hidden md:table-cell px-4 py-3">Description</th>
              <th className="hidden sm:table-cell px-4 py-3">Created</th>
              <th className="px-4 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {loading ? (
              Array.from({ length: 4 }).map((_, i) => (
                <tr key={i} className="bg-gray-950">
                  <td className="px-4 py-3"><Skeleton className="h-4 w-32" /></td>
                  <td className="hidden md:table-cell px-4 py-3"><Skeleton className="h-4 w-48" /></td>
                  <td className="hidden sm:table-cell px-4 py-3"><Skeleton className="h-4 w-24" /></td>
                  <td className="px-4 py-3"><Skeleton className="h-4 w-16 ml-auto" /></td>
                </tr>
              ))
            ) : rooms.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-4 py-8 text-center text-gray-500 bg-gray-950">
                  No public rooms found
                </td>
              </tr>
            ) : (
              rooms.map((room) => (
                <tr key={room.id} className="bg-gray-950 hover:bg-gray-900">
                  <td className="px-4 py-3 font-medium text-gray-100">
                    <span className="inline-flex items-center gap-2">
                      {room.name}
                      <span className="rounded-full bg-emerald-500/10 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-emerald-400">
                        Public
                      </span>
                    </span>
                  </td>
                  <td className="hidden md:table-cell px-4 py-3 text-gray-400 max-w-xs truncate">
                    {room.description || <span className="text-gray-600 italic">No description</span>}
                  </td>
                  <td className="hidden sm:table-cell px-4 py-3 text-gray-400">
                    {new Date(room.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <Button
                      size="sm"
                      variant="danger"
                      onClick={() => { setDeleteTarget(room); setDeleteError("") }}
                    >
                      Delete
                    </Button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {createOpen && session?.accessToken && (
        <CreateModal
          token={session.accessToken}
          onDone={() => { setCreateOpen(false); fetchRooms() }}
          onClose={() => setCreateOpen(false)}
        />
      )}

      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="w-full max-w-sm rounded-lg bg-gray-900 ring-1 ring-gray-700 p-6 space-y-4">
            <h2 className="text-base font-semibold text-foreground">Delete {deleteTarget.name}?</h2>
            <p className="text-sm text-gray-400">
              This will permanently delete the public room and all its messages. This action cannot be undone.
            </p>
            {deleteError && <p className="text-sm text-red-400">{deleteError}</p>}
            <div className="flex justify-end gap-3 pt-2">
              <Button variant="secondary" onClick={() => setDeleteTarget(null)}>Cancel</Button>
              <Button variant="danger" loading={deleteLoading} onClick={confirmDelete}>Delete</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
