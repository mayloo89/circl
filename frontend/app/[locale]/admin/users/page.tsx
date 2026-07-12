"use client"

import { useSession } from "next-auth/react"
import { useLocale, useTranslations } from "next-intl"
import { useEffect, useRef, useState } from "react"
import { Link } from "@/i18n/navigation"
import Button from "@/components/ui/Button"
import Input from "@/components/ui/Input"
import Skeleton from "@/components/ui/Skeleton"
import { formatLastSeen } from "@/hooks/usePresence"
import { useMenuKeyboard } from "@/hooks/useMenuKeyboard"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface UserRecord {
  id: string
  email: string
  username: string
  display_name: string
  status: string
  role: string
  created_at: string
  online: boolean
  last_seen_at: string | null
}

const STATUS_OPTIONS = ["", "active", "suspended", "banned"]

const STATUS_BADGE: Record<string, string> = {
  active: "bg-green-900 text-green-300",
  suspended: "bg-yellow-900 text-yellow-300",
  banned: "bg-red-900 text-red-300",
}

const ROLE_BADGE: Record<string, string> = {
  super_admin: "text-yellow-400",
  admin: "text-brand-muted",
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
  const t = useTranslations("admin")
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
        setError(text.trim() || t("suspendFailed"))
        return
      }
      onDone()
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/80">
      <div className="w-full max-w-md rounded-lg bg-gray-900 ring-1 ring-gray-700 p-6 space-y-4">
        <h2 className="text-base font-semibold text-foreground">{t("suspendTitle", { email: user.email })}</h2>
        {error && <p className="text-sm text-red-400">{error}</p>}
        <div>
          <label className="block text-xs text-gray-400 mb-1">{t("suspendDurationLabel")}</label>
          <Input
            type="number"
            min="0"
            value={days}
            onChange={(e) => setDays(e.target.value)}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-400 mb-1">{t("suspendReasonLabel")}</label>
          <Input
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder={t("suspendReasonPlaceholder")}
          />
        </div>
        <div className="flex justify-end gap-3 pt-2">
          <Button variant="secondary" onClick={onClose}>{t("cancel")}</Button>
          <Button variant="warning" loading={loading} onClick={submit}>{t("suspend")}</Button>
        </div>
      </div>
    </div>
  )
}

// Modal for hard delete — super_admin only
function HardDeleteModal({
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
  const t = useTranslations("admin")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  async function confirm() {
    setLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/admin/users/${user.id}`, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) {
        const text = await res.text()
        setError(text.trim() || t("deleteFailed"))
        return
      }
      onDone()
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/80">
      <div className="w-full max-w-sm rounded-lg bg-gray-900 ring-1 ring-gray-700 p-6 space-y-4">
        <h2 className="text-base font-semibold text-foreground">{t("hardDeleteTitle")}</h2>
        <p className="text-sm text-gray-400">
          {t("hardDeleteWarningPre")} <span className="text-gray-200">{user.email}</span>{t("hardDeleteWarningPost")}
        </p>
        {error && <p className="text-sm text-red-400">{error}</p>}
        <div className="flex justify-end gap-3 pt-2">
          <Button variant="secondary" onClick={onClose}>{t("cancel")}</Button>
          <Button variant="danger" loading={loading} onClick={confirm}>{t("deletePermanently")}</Button>
        </div>
      </div>
    </div>
  )
}

// Modal for changing role — super_admin only
function RoleModal({
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
  const t = useTranslations("admin")
  const [role, setRole] = useState(user.role)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  async function submit() {
    setLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/admin/users/${user.id}/role`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ role }),
      })
      if (!res.ok) {
        const text = await res.text()
        setError(text.trim() || t("roleFailed"))
        return
      }
      onDone()
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/80">
      <div className="w-full max-w-sm rounded-lg bg-gray-900 ring-1 ring-gray-700 p-6 space-y-4">
        <h2 className="text-base font-semibold text-foreground">{t("changeRoleTitle", { email: user.email })}</h2>
        {error && <p className="text-sm text-red-400">{error}</p>}
        <div className="flex gap-3">
          {(["user", "admin", "super_admin"] as const).map((r) => (
            <button
              key={r}
              onClick={() => setRole(r)}
              className={`rounded px-3 py-1.5 text-sm font-medium transition-colors ${
                role === r
                  ? "bg-brand-primary text-foreground"
                  : "bg-gray-800 text-gray-300 hover:bg-gray-700"
              }`}
            >
              {r === "super_admin" ? t("roleSuperAdmin") : r === "admin" ? t("adminBadge") : t("roleUser")}
            </button>
          ))}
        </div>
        <div className="flex justify-end gap-3 pt-2">
          <Button variant="secondary" onClick={onClose}>{t("cancel")}</Button>
          <Button variant="primary" loading={loading} onClick={submit}>{t("save")}</Button>
        </div>
      </div>
    </div>
  )
}

function ActionsMenu({
  user,
  isSuperAdmin,
  actionLoading,
  onSuspend,
  onBan,
  onReactivate,
  onRole,
  onHardDelete,
}: {
  user: UserRecord
  isSuperAdmin: boolean
  actionLoading: string | null
  onSuspend: () => void
  onBan: () => void
  onReactivate: () => void
  onRole: () => void
  onHardDelete: () => void
}) {
  const [open, setOpen] = useState(false)
  const wrapperRef = useRef<HTMLDivElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)

  useMenuKeyboard({ open, containerRef: menuRef, onClose: () => setOpen(false) })

  useEffect(() => {
    if (!open) return
    function handleOutside(e: MouseEvent) {
      if (wrapperRef.current && !wrapperRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener("mousedown", handleOutside)
    return () => document.removeEventListener("mousedown", handleOutside)
  }, [open])

  const isLoading = !!actionLoading?.startsWith(user.id)

  const t = useTranslations("admin")

  const statusItems: { label: string; onClick: () => void; cls: string }[] = []
  if (user.status === "active") {
    statusItems.push({ label: t("suspend"), onClick: () => { setOpen(false); onSuspend() }, cls: "text-yellow-400" })
    statusItems.push({ label: t("ban"), onClick: () => { setOpen(false); onBan() }, cls: "text-red-400" })
  }
  if (user.status === "suspended" || user.status === "banned") {
    statusItems.push({ label: t("reactivate"), onClick: () => { setOpen(false); onReactivate() }, cls: "text-green-400" })
  }

  const adminItems: { label: string; onClick: () => void; cls: string }[] = []
  if (isSuperAdmin) {
    adminItems.push({ label: t("changeRoleAction"), onClick: () => { setOpen(false); onRole() }, cls: "text-gray-200" })
    adminItems.push({ label: t("adminChannelDelete"), onClick: () => { setOpen(false); onHardDelete() }, cls: "text-red-400" })
  }

  const allItems = [...statusItems, ...adminItems]
  if (allItems.length === 0) return null

  const triggerId = `actions-trigger-${user.id}`

  return (
    <div ref={wrapperRef} className="relative inline-block">
      <Button
        id={triggerId}
        size="sm"
        variant="secondary"
        aria-haspopup="menu"
        aria-expanded={open}
        disabled={isLoading}
        onClick={() => setOpen((v) => !v)}
      >
        {t("actionsMenu")}
        <svg
          className={`h-3 w-3 transition-transform duration-150 ${open ? "rotate-180" : ""}`}
          viewBox="0 0 12 12"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          aria-hidden="true"
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M2 4l4 4 4-4" />
        </svg>
      </Button>

      {open && (
        <div
          ref={menuRef}
          role="menu"
          aria-labelledby={triggerId}
          className="absolute right-0 top-full mt-1 z-20 min-w-[10rem] rounded-lg bg-gray-900 py-1 ring-1 ring-gray-700 shadow-lg"
        >
          {statusItems.map((item) => (
            <button
              key={item.label}
              role="menuitem"
              onClick={item.onClick}
              className={`block w-full text-left px-4 py-2 text-sm transition-colors hover:bg-gray-800 focus:bg-gray-800 focus:outline-none ${item.cls}`}
            >
              {item.label}
            </button>
          ))}
          {statusItems.length > 0 && adminItems.length > 0 && (
            <div className="my-1 border-t border-gray-800" role="separator" />
          )}
          {adminItems.map((item) => (
            <button
              key={item.label}
              role="menuitem"
              onClick={item.onClick}
              className={`block w-full text-left px-4 py-2 text-sm transition-colors hover:bg-gray-800 focus:bg-gray-800 focus:outline-none ${item.cls}`}
            >
              {item.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

export default function AdminUsersPage() {
  const { data: session } = useSession()
  const t = useTranslations("admin")
  const locale = useLocale()
  const [users, setUsers] = useState<UserRecord[]>([])
  const [total, setTotal] = useState(0)
  const [query, setQuery] = useState("")
  const [status, setStatus] = useState("")
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [suspendTarget, setSuspendTarget] = useState<UserRecord | null>(null)
  const [hardDeleteTarget, setHardDeleteTarget] = useState<UserRecord | null>(null)
  const [roleTarget, setRoleTarget] = useState<UserRecord | null>(null)
  const [actionLoading, setActionLoading] = useState<string | null>(null)

  const isSuperAdmin = session?.role === "super_admin"
  const limit = 20

  const [refreshKey, setRefreshKey] = useState(0)

  // Show the loading skeleton on every refetch (search, status filter,
  // pagination or refresh) without a synchronous setState in the effect.
  const fetchKey = `${query}|${status}|${offset}|${refreshKey}`
  const [loadingKey, setLoadingKey] = useState(fetchKey)
  if (loadingKey !== fetchKey) {
    setLoadingKey(fetchKey)
    setLoading(true)
  }

  useEffect(() => {
    const token = session?.accessToken
    if (!token) return
    let cancelled = false
    ;(async () => {
      try {
        const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
        if (query) params.set("q", query)
        if (status) params.set("status", status)

        const res = await fetch(`${API_URL}/admin/users?${params}`, {
          headers: { Authorization: `Bearer ${token}` },
        })
        if (!res.ok) throw new Error(String(res.status))
        const data = await res.json()
        if (cancelled) return
        setUsers(data.users ?? [])
        setTotal(data.total ?? 0)
        setError("")
      } catch {
        if (!cancelled) setError(t("loadUsersFailed"))
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => { cancelled = true }
  }, [session, query, status, offset, t, refreshKey])

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
        setError(text.trim() || (action === "ban" ? t("banFailed") : t("reactivateFailed")))
        return
      }
      setRefreshKey((k) => k + 1)
    } finally {
      setActionLoading(null)
    }
  }

  function handleSearch(e: React.FormEvent) {
    e.preventDefault()
    setOffset(0)
    setRefreshKey((k) => k + 1)
  }

  const totalPages = Math.ceil(total / limit)
  const currentPage = Math.floor(offset / limit) + 1

  return (
    <div className="p-4 sm:p-6 md:p-8">
      <h1 className="text-2xl font-bold text-foreground mb-6">{t("usersTitle")}</h1>

      {/* Search + filter */}
      <form onSubmit={handleSearch} className="flex flex-wrap gap-3 mb-6">
        <Input
          className="w-full sm:w-64"
          placeholder={t("usersSearchPlaceholder")}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <select
          value={status}
          onChange={(e) => { setStatus(e.target.value); setOffset(0) }}
          className="rounded bg-gray-800 px-3 py-2 text-sm text-gray-200 ring-1 ring-gray-700 focus:outline-none"
        >
          {STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>{s || t("usersStatusAll")}</option>
          ))}
        </select>
        <Button type="submit" variant="secondary" size="sm">{t("searchBtn")}</Button>
      </form>

      {error && <p className="mb-4 text-sm text-red-400">{error}</p>}

      {/* Table */}
      <div className="overflow-x-auto rounded-lg ring-1 ring-gray-800">
        <table className="w-full text-sm text-left">
          <thead className="bg-gray-900 text-xs uppercase tracking-wider text-gray-500">
            <tr>
              <th className="px-4 py-3">{t("usersColUser")}</th>
              <th className="px-4 py-3">{t("usersColStatus")}</th>
              <th className="hidden md:table-cell px-4 py-3">{t("usersColActivity")}</th>
              <th className="hidden sm:table-cell px-4 py-3">{t("usersColJoined")}</th>
              <th className="px-4 py-3 text-right">{t("actionsMenu")}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {loading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <tr key={i} className="bg-gray-950">
                  <td className="px-4 py-3"><Skeleton className="h-4 w-48" /></td>
                  <td className="px-4 py-3"><Skeleton className="h-4 w-20" /></td>
                  <td className="hidden md:table-cell px-4 py-3"><Skeleton className="h-4 w-28" /></td>
                  <td className="hidden sm:table-cell px-4 py-3"><Skeleton className="h-4 w-24" /></td>
                  <td className="px-4 py-3"><Skeleton className="h-4 w-32 ml-auto" /></td>
                </tr>
              ))
            ) : users.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-gray-500 bg-gray-950">
                  {t("usersNoFound")}
                </td>
              </tr>
            ) : (
              users.map((u) => (
                <tr key={u.id} className="bg-gray-950 hover:bg-gray-900">
                  <td className="px-4 py-3">
                    {u.username ? (
                      <Link
                        href={`/profile/${u.username}`}
                        className="text-gray-100 font-medium hover:text-brand-muted transition-colors"
                      >
                        {u.display_name || u.username}
                      </Link>
                    ) : (
                      <p className="text-gray-100 font-medium">{u.display_name || "—"}</p>
                    )}
                    <p className="text-xs text-gray-500">{u.email}</p>
                    {u.role !== "user" && (
                      <span className={`text-xs font-medium ${ROLE_BADGE[u.role] ?? "text-gray-400"}`}>
                        {u.role === "super_admin" ? t("roleSuperAdmin") : u.role}
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_BADGE[u.status] ?? "bg-gray-800 text-gray-400"}`}>
                      {u.status}
                    </span>
                  </td>
                  <td className="hidden md:table-cell px-4 py-3 text-gray-400">
                    {u.online ? (
                      <span className="inline-flex items-center gap-2">
                        <span className="h-2 w-2 rounded-full bg-green-500" aria-hidden="true" />
                        <span className="text-green-400">{t("usersOnline")}</span>
                      </span>
                    ) : u.last_seen_at ? (
                      <span className="inline-flex items-center gap-2">
                        <span className="h-2 w-2 rounded-full bg-gray-600" aria-hidden="true" />
                        <span title={new Date(u.last_seen_at).toLocaleString(locale)}>{formatLastSeen(u.last_seen_at)}</span>
                      </span>
                    ) : (
                      <span className="text-gray-600">{t("usersNeverSeen")}</span>
                    )}
                  </td>
                  <td className="hidden sm:table-cell px-4 py-3 text-gray-400">
                    {new Date(u.created_at).toLocaleDateString(locale)}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <ActionsMenu
                      user={u}
                      isSuperAdmin={isSuperAdmin}
                      actionLoading={actionLoading}
                      onSuspend={() => setSuspendTarget(u)}
                      onBan={() => doAction(u, "ban")}
                      onReactivate={() => doAction(u, "reactivate")}
                      onRole={() => setRoleTarget(u)}
                      onHardDelete={() => setHardDeleteTarget(u)}
                    />
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
          <span>{t("usersTotalCount", { count: total })}</span>
          <div className="flex gap-2">
            <Button
              size="sm"
              variant="secondary"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - limit))}
            >
              {t("usersPrev")}
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
              {t("usersNext")}
            </Button>
          </div>
        </div>
      )}

      {suspendTarget && session?.accessToken && (
        <SuspendModal
          user={suspendTarget}
          token={session.accessToken}
          onDone={() => { setSuspendTarget(null); setRefreshKey((k) => k + 1) }}
          onClose={() => setSuspendTarget(null)}
        />
      )}

      {hardDeleteTarget && session?.accessToken && (
        <HardDeleteModal
          user={hardDeleteTarget}
          token={session.accessToken}
          onDone={() => { setHardDeleteTarget(null); setRefreshKey((k) => k + 1) }}
          onClose={() => setHardDeleteTarget(null)}
        />
      )}

      {roleTarget && session?.accessToken && (
        <RoleModal
          user={roleTarget}
          token={session.accessToken}
          onDone={() => { setRoleTarget(null); setRefreshKey((k) => k + 1) }}
          onClose={() => setRoleTarget(null)}
        />
      )}
    </div>
  )
}
