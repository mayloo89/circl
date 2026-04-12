"use client"

import { useSession } from "next-auth/react"
import { useRouter } from "next/navigation"
import { useEffect, useState } from "react"

import Avatar from "@/components/ui/Avatar"
import Button from "@/components/ui/Button"
import Input from "@/components/ui/Input"
import Modal from "@/components/ui/Modal"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface ChannelSummary {
  id: string
  name: string
  description: string
  creator_id: string
  active_count: number
  created_at: string
}

function ChannelSkeleton() {
  return (
    <li className="flex items-center gap-4 px-5 py-4">
      <Skeleton className="h-10 w-10 flex-none rounded-full" />
      <div className="flex-1 space-y-2">
        <Skeleton className="h-3.5 w-28" />
        <Skeleton className="h-3 w-56 bg-gray-800/70" />
      </div>
      <Skeleton className="h-8 w-16 rounded" />
    </li>
  )
}

interface CreateChannelModalProps {
  open: boolean
  token: string
  onClose: () => void
  onCreated: (channel: ChannelSummary) => void
}

function CreateChannelModal({ open, token, onClose, onCreated }: CreateChannelModalProps) {
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  async function handleCreate() {
    if (!name.trim()) { setError("Name is required."); return }
    setLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/chat/channels`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({ name: name.trim(), description: description.trim() }),
      })
      if (!res.ok) { setError("Failed to create channel."); return }
      const room = await res.json()
      setName(""); setDescription("")
      onCreated({ ...room } as ChannelSummary)
    } catch {
      setError("Failed to create channel.")
    } finally {
      setLoading(false)
    }
  }

  function handleClose() {
    setName(""); setDescription(""); setError("")
    onClose()
  }

  return (
    <Modal open={open} onClose={handleClose}>
      <div
        className="w-full max-w-md rounded-xl bg-gray-900 shadow-2xl ring-1 ring-gray-700"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-gray-800 px-5 py-4">
          <h2 className="text-base font-semibold text-white">New channel</h2>
          <button type="button" onClick={handleClose} className="text-gray-500 hover:text-gray-300" aria-label="Close">✕</button>
        </div>
        <div className="space-y-4 p-5">
          <Input label="Channel name" placeholder="e.g. general" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
          <Input label="Description (optional)" placeholder="What's this channel about?" value={description} onChange={(e) => setDescription(e.target.value)} />
          {error && <p className="text-xs text-red-400">{error}</p>}
        </div>
        <div className="flex justify-end gap-3 border-t border-gray-800 px-5 py-4">
          <Button variant="ghost" size="sm" onClick={handleClose} disabled={loading}>Cancel</Button>
          <Button variant="primary" size="sm" onClick={handleCreate} loading={loading} disabled={!name.trim()}>
            Create channel
          </Button>
        </div>
      </div>
    </Modal>
  )
}

export default function ChannelsPage() {
  const { data: session, status } = useSession()
  const router = useRouter()
  const [channels, setChannels] = useState<ChannelSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [createOpen, setCreateOpen] = useState(false)
  const [query, setQuery] = useState("")

  const token = session?.accessToken
  const isAdmin = session?.role === "admin" || session?.role === "super_admin"

  function loadChannels() {
    if (!token) return
    setLoading(true)
    setError("")
    fetch(`${API_URL}/chat/channels`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => r.json())
      .then((data: ChannelSummary[]) => setChannels(data))
      .catch(() => setError("Failed to load channels."))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    if (status !== "authenticated" || !token) return
    loadChannels()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, token])

  const filtered = channels.filter(
    (c) => !query || c.name.toLowerCase().includes(query.toLowerCase()) || c.description.toLowerCase().includes(query.toLowerCase())
  )

  return (
    <div className="flex min-h-screen flex-col items-center bg-gray-950 py-10">
      {token && (
        <CreateChannelModal
          open={createOpen}
          token={token}
          onClose={() => setCreateOpen(false)}
          onCreated={(ch) => {
            setChannels((prev) => [ch, ...prev])
            setCreateOpen(false)
            router.push(`/chat/${ch.id}`)
          }}
        />
      )}

      <div className="w-full max-w-2xl space-y-6 px-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold text-white">Channels</h1>
            <p className="mt-1 text-sm text-gray-500">Public rooms anyone can join and chat in.</p>
          </div>
          <div className="flex items-center gap-2">
            {isAdmin && (
              <Button variant="primary" size="sm" onClick={() => setCreateOpen(true)}>
                + New channel
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={() => router.push("/chat")}>← Messages</Button>
          </div>
        </div>

        <Input
          labelHidden
          label="Search channels"
          placeholder="Search channels…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />

        {error && (
          <div className="flex items-center justify-between rounded-md bg-red-950 p-3 ring-1 ring-red-900">
            <p className="text-sm text-red-400">{error}</p>
            <Button variant="danger" size="sm" onClick={loadChannels} className="ml-3 shrink-0">Retry</Button>
          </div>
        )}

        <div className="rounded-lg bg-gray-900 shadow-xl ring-1 ring-gray-800">
          {status === "loading" || loading ? (
            <ul className="divide-y divide-gray-800">
              {[0, 1, 2, 3].map((i) => <ChannelSkeleton key={i} />)}
            </ul>
          ) : filtered.length === 0 ? (
            <div className="flex flex-col items-center gap-3 px-6 py-14 text-center">
              <div className="flex h-14 w-14 items-center justify-center rounded-full bg-gray-800">
                <svg className="h-7 w-7 text-gray-600" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 8.25h9m-9 3H12m-9.75 1.51c0 1.6 1.123 2.994 2.707 3.227 1.129.166 2.27.293 3.423.379.35.026.67.21.865.501L12 21l2.755-4.133a1.14 1.14 0 01.865-.501 48.172 48.172 0 003.423-.379c1.584-.233 2.707-1.626 2.707-3.228V6.741c0-1.602-1.123-2.995-2.707-3.228A48.394 48.394 0 0012 3c-2.392 0-4.744.175-7.043.513C3.373 3.746 2.25 5.14 2.25 6.741v6.018z" />
                </svg>
              </div>
              <p className="text-sm font-medium text-gray-300">
                {query ? "No channels match your search." : "No channels yet."}
              </p>
              {!query && isAdmin && (
                <Button variant="primary" size="sm" pill onClick={() => setCreateOpen(true)} className="mt-1">
                  Create the first one
                </Button>
              )}
            </div>
          ) : (
            <ul className="divide-y divide-gray-800">
              {filtered.map((ch) => (
                <li key={ch.id} className="flex items-center gap-4 px-5 py-4">
                  <Avatar name={ch.name} size="md" color="indigo" />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-semibold text-white"># {ch.name}</p>
                    {ch.description && (
                      <p className="mt-0.5 truncate text-xs text-gray-400">{ch.description}</p>
                    )}
                    {ch.active_count > 0 && (
                      <p className="mt-0.5 text-xs text-gray-600">{ch.active_count} online now</p>
                    )}
                  </div>
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={() => router.push(`/chat/${ch.id}`)}
                  >
                    Enter
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  )
}
