"use client"

import { useSession } from "next-auth/react"
import { useCallback, useEffect, useState } from "react"
import Button from "@/components/ui/Button"
import Input from "@/components/ui/Input"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface UserRecord {
  id: string
  email: string
  username: string
  display_name: string
  status: string
  is_admin: boolean
  created_at: string
}

const STATUS_OPTIONS = ["", "active", "suspended", "banned"]

const STATUS_BADGE: Record<string, string> = {
  active: "bg-green-900 text-green-300",
  suspended: "bg-yellow-900 text-yellow-300",
  banned: "bg-red-900 text-red-300",
}

// Modal for suspend action (needs duration + reason)
function SuspendModal({
  user,
  token,
  onDone,
  onClose,
}: {
  user: UserRecord
  token: string
  onDone: () => void
  onClose: () => void
}) {
  const [reason, setReason] = useState("")
  const [days, setDays] = useState("7")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  async function submit() {
    setLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/admin/users/${user.id}/status`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          action: "suspend",
          reason,
          duration_days: parseInt(days, 10) || 0,
        }),
      })
      if (!res.ok) {
        const text = await res.text()
        setError(text.trim() || "Failed to suspend user")
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
        <h2 className="text-base font-semibold text-white">Suspend {user.email}</h2>
        {error && <p className="text-sm text-red-400">{error}</p>}
        <div>
          <label className="block text-xs text-gray-400 mb-1">Duration (days, 0 = permanent)</label>
          <Input
            type="number"
            min="0"
            value={days}
            onChange={(e) => setDays(e.target.value)}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-400 mb-1">Reason</label>
          <Input
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Reason for suspension"
          />
        </div>
        <div className="flex justify-end gap-3 pt-2">
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button variant="warning" loading={loading} onClick={submit}>Suspend</Button>
        </div>
      </div>
    </div>
  )
}

export default function AdminUsersPage() {
  const { data: session } = useSession()
  const [users, setUsers] = useState<UserRecord[]>([])
  const [total, setTotal] = useState(0)
  const [query, setQuery] = useState("")
  const [status, setStatus] = useState("")
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const [suspendTarget, setSuspendTarget] = useState<UserRecord | null>(null)
  const [actionLoading, setActionLoading] = useState<string | null>(null)

  const limit = 20

  const fetchUsers = useCallback(async () => {
    if (!session?.accessToken) return
    setLoading(true)
    setError("")
    try {
      const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
      if (query) params.set("q", query)
      if (status) params.set("status", status)

      const res = await fetch(`${API_URL}/admin/users?${params}`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      })
      if (!res.ok) throw new Error(String(res.status))
      const data = await res.json()
      setUsers(data.users ?? [])
      setTotal(data.total ?? 0)
    } catch {
      setError("Failed to load users")
    } finally {
      setLoading(false)
    }
  }, [session, query, status, offset])

  useEffect(() => { fetchUsers() }, [fetchUsers])

  async function doAction(user: UserRecord, action: "ban" | "reactivate") {
    if (!session?.accessToken) return
    setActionLoading(user.id + action)
    try {
      const res = await fetch(`${API_URL}/admin/users/${user.id}/status`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${session.accessToken}`,
        },
        body: JSON.stringify({ action }),
      })
      if (!res.ok) {
        const text = await res.text()
        setError(text.trim() || `Failed to ${action} user`)
        return
      }
      await fetchUsers()
    } finally {
      setActionLoading(null)
    }
  }

  function handleSearch(e: React.FormEvent) {
    e.preventDefault()
    setOffset(0)
    fetchUsers()
  }

  const totalPages = Math.ceil(total / limit)
  const currentPage = Math.floor(offset / limit) + 1

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold text-white mb-6">Users</h1>

      {/* Search + filter */}
      <form onSubmit={handleSearch} className="flex flex-wrap gap-3 mb-6">
        <Input
          className="w-64"
          placeholder="Search by email or username"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <select
          value={status}
          onChange={(e) => { setStatus(e.target.value); setOffset(0) }}
          className="rounded bg-gray-800 px-3 py-2 text-sm text-gray-200 ring-1 ring-gray-700 focus:outline-none"
        >
          {STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>{s || "All statuses"}</option>
          ))}
        </select>
        <Button type="submit" variant="secondary" size="sm">Search</Button>
      </form>

      {error && <p className="mb-4 text-sm text-red-400">{error}</p>}

      {/* Table */}
      <div className="overflow-x-auto rounded-lg ring-1 ring-gray-800">
        <table className="w-full text-sm text-left">
          <thead className="bg-gray-900 text-xs uppercase tracking-wider text-gray-500">
            <tr>
              <th className="px-4 py-3">User</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3">Joined</th>
              <th className="px-4 py-3 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {loading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <tr key={i} className="bg-gray-950">
                  <td className="px-4 py-3"><Skeleton className="h-4 w-48" /></td>
                  <td className="px-4 py-3"><Skeleton className="h-4 w-20" /></td>
                  <td className="px-4 py-3"><Skeleton className="h-4 w-24" /></td>
                  <td className="px-4 py-3"><Skeleton className="h-4 w-32 ml-auto" /></td>
                </tr>
              ))
            ) : users.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-4 py-8 text-center text-gray-500 bg-gray-950">
                  No users found
                </td>
              </tr>
            ) : (
              users.map((u) => (
                <tr key={u.id} className="bg-gray-950 hover:bg-gray-900">
                  <td className="px-4 py-3">
                    <p className="text-gray-100 font-medium">{u.display_name || u.username || "—"}</p>
                    <p className="text-xs text-gray-500">{u.email}</p>
                    {u.is_admin && (
                      <span className="text-xs text-indigo-400 font-medium">admin</span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_BADGE[u.status] ?? "bg-gray-800 text-gray-400"}`}>
                      {u.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-400">
                    {new Date(u.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-2">
                      {(u.status === "active") && (
                        <>
                          <Button
                            size="sm"
                            variant="warning"
                            loading={actionLoading === u.id + "suspend"}
                            onClick={() => setSuspendTarget(u)}
                          >
                            Suspend
                          </Button>
                          <Button
                            size="sm"
                            variant="danger"
                            loading={actionLoading === u.id + "ban"}
                            onClick={() => doAction(u, "ban")}
                          >
                            Ban
                          </Button>
                        </>
                      )}
                      {(u.status === "suspended" || u.status === "banned") && (
                        <Button
                          size="sm"
                          variant="success"
                          loading={actionLoading === u.id + "reactivate"}
                          onClick={() => doAction(u, "reactivate")}
                        >
                          Reactivate
                        </Button>
                      )}
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="mt-4 flex items-center justify-between text-sm text-gray-400">
          <span>{total} users total</span>
          <div className="flex gap-2">
            <Button
              size="sm"
              variant="secondary"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - limit))}
            >
              Previous
            </Button>
            <span className="flex items-center px-2">
              {currentPage} / {totalPages}
            </span>
            <Button
              size="sm"
              variant="secondary"
              disabled={offset + limit >= total}
              onClick={() => setOffset(offset + limit)}
            >
              Next
            </Button>
          </div>
        </div>
      )}

      {suspendTarget && session?.accessToken && (
        <SuspendModal
          user={suspendTarget}
          token={session.accessToken}
          onDone={() => { setSuspendTarget(null); fetchUsers() }}
          onClose={() => setSuspendTarget(null)}
        />
      )}
    </div>
  )
}
