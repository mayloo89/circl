"use client"

import { useSession } from "next-auth/react"
import { useLocale, useTranslations } from "next-intl"
import { useCallback, useEffect, useState } from "react"
import Button from "@/components/ui/Button"
import Input from "@/components/ui/Input"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface Channel {
  id: string
  name: string
  description: string
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
  const t = useTranslations("admin")
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  async function submit() {
    setLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/admin/channels`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ name, description }),
      })
      if (!res.ok) {
        const text = await res.text()
        setError(text.trim() || t("createAdminChannelFailed"))
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
        <h2 className="text-base font-semibold text-foreground">{t("adminChannelCreateTitle")}</h2>
        {error && <p className="text-sm text-red-400">{error}</p>}
        <div>
          <label className="block text-xs text-gray-400 mb-1">{t("adminChannelNameLabel")}</label>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("adminChannelNamePlaceholder")}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-400 mb-1">{t("adminChannelDescriptionLabel")}</label>
          <Input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder={t("adminChannelDescriptionPlaceholder")}
          />
        </div>
        <div className="flex justify-end gap-3 pt-2">
          <Button variant="secondary" onClick={onClose}>{t("cancel")}</Button>
          <Button variant="primary" loading={loading} onClick={submit}>{t("adminChannelCreate")}</Button>
        </div>
      </div>
    </div>
  )
}

function EditModal({
  channel,
  token,
  onDone,
  onClose,
}: {
  channel: Channel
  token: string
  onDone: () => void
  onClose: () => void
}) {
  const t = useTranslations("admin")
  const [name, setName] = useState(channel.name)
  const [description, setDescription] = useState(channel.description)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  async function submit() {
    setLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/admin/channels/${channel.id}`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ name, description }),
      })
      if (!res.ok) {
        const text = await res.text()
        setError(text.trim() || t("updateAdminChannelFailed"))
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
        <h2 className="text-base font-semibold text-foreground">{t("adminChannelEditTitle", { name: channel.name })}</h2>
        {error && <p className="text-sm text-red-400">{error}</p>}
        <div>
          <label className="block text-xs text-gray-400 mb-1">{t("adminChannelNameLabel")}</label>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-400 mb-1">{t("adminChannelDescriptionLabel")}</label>
          <Input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder={t("adminChannelDescriptionPlaceholder")}
          />
        </div>
        <div className="flex justify-end gap-3 pt-2">
          <Button variant="secondary" onClick={onClose}>{t("cancel")}</Button>
          <Button variant="primary" loading={loading} onClick={submit}>{t("save")}</Button>
        </div>
      </div>
    </div>
  )
}

export default function AdminChannelsPage() {
  const { data: session } = useSession()
  const t = useTranslations("admin")
  const locale = useLocale()
  const [channels, setChannels] = useState<Channel[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<Channel | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Channel | null>(null)
  const [deleteLoading, setDeleteLoading] = useState(false)
  const [deleteError, setDeleteError] = useState("")

  const fetchChannels = useCallback(async () => {
    if (!session?.accessToken) return
    setLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/admin/channels`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      })
      if (!res.ok) throw new Error(String(res.status))
      const data = await res.json()
      setChannels(Array.isArray(data) ? data : [])
    } catch {
      setError(t("loadAdminChannelsFailed"))
    } finally {
      setLoading(false)
    }
  }, [session, t])

  useEffect(() => { fetchChannels() }, [fetchChannels])

  async function confirmDelete() {
    if (!deleteTarget || !session?.accessToken) return
    setDeleteLoading(true)
    setDeleteError("")
    try {
      const res = await fetch(`${API_URL}/admin/channels/${deleteTarget.id}`, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${session.accessToken}` },
      })
      if (!res.ok) {
        const text = await res.text()
        setDeleteError(text.trim() || t("deleteAdminChannelFailed"))
        return
      }
      setDeleteTarget(null)
      await fetchChannels()
    } finally {
      setDeleteLoading(false)
    }
  }

  return (
    <div className="p-4 sm:p-6 md:p-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-foreground">{t("adminChannelsTitle")}</h1>
        <Button variant="primary" size="sm" onClick={() => setCreateOpen(true)}>
          {t("adminChannelCreate")}
        </Button>
      </div>

      {error && <p className="mb-4 text-sm text-red-400">{error}</p>}

      <div className="overflow-x-auto rounded-lg ring-1 ring-gray-800">
        <table className="w-full text-sm text-left">
          <thead className="bg-gray-900 text-xs uppercase tracking-wider text-gray-500">
            <tr>
              <th className="px-4 py-3">{t("adminChannelNameLabel")}</th>
              <th className="hidden md:table-cell px-4 py-3">{t("adminChannelDescriptionLabel")}</th>
              <th className="hidden sm:table-cell px-4 py-3">{t("colDate")}</th>
              <th className="px-4 py-3 text-right">{t("actionsMenu")}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {loading ? (
              Array.from({ length: 4 }).map((_, i) => (
                <tr key={i} className="bg-gray-950">
                  <td className="px-4 py-3"><Skeleton className="h-4 w-32" /></td>
                  <td className="hidden md:table-cell px-4 py-3"><Skeleton className="h-4 w-48" /></td>
                  <td className="hidden sm:table-cell px-4 py-3"><Skeleton className="h-4 w-24" /></td>
                  <td className="px-4 py-3"><Skeleton className="h-4 w-24 ml-auto" /></td>
                </tr>
              ))
            ) : channels.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-4 py-8 text-center text-gray-500 bg-gray-950">
                  {t("adminChannelNoFound")}
                </td>
              </tr>
            ) : (
              channels.map((c) => (
                <tr key={c.id} className="bg-gray-950 hover:bg-gray-900">
                  <td className="px-4 py-3 font-medium text-gray-100">#{c.name}</td>
                  <td className="hidden md:table-cell px-4 py-3 text-gray-400 max-w-xs truncate">
                    {c.description || <span className="text-gray-600 italic">{t("adminChannelNoDescription")}</span>}
                  </td>
                  <td className="hidden sm:table-cell px-4 py-3 text-gray-400">
                    {new Date(c.created_at).toLocaleDateString(locale)}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-2">
                      <Button
                        size="sm"
                        variant="secondary"
                        onClick={() => setEditTarget(c)}
                      >
                        {t("adminChannelEdit")}
                      </Button>
                      <Button
                        size="sm"
                        variant="danger"
                        onClick={() => { setDeleteTarget(c); setDeleteError("") }}
                      >
                        {t("adminChannelDelete")}
                      </Button>
                    </div>
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
          onDone={() => { setCreateOpen(false); fetchChannels() }}
          onClose={() => setCreateOpen(false)}
        />
      )}

      {editTarget && session?.accessToken && (
        <EditModal
          channel={editTarget}
          token={session.accessToken}
          onDone={() => { setEditTarget(null); fetchChannels() }}
          onClose={() => setEditTarget(null)}
        />
      )}

      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/80">
          <div className="w-full max-w-sm rounded-lg bg-gray-900 ring-1 ring-gray-700 p-6 space-y-4">
            <h2 className="text-base font-semibold text-foreground">{t("adminChannelDeleteTitle", { name: deleteTarget.name })}</h2>
            <p className="text-sm text-gray-400">
              {t("adminChannelDeleteWarning")}
            </p>
            {deleteError && <p className="text-sm text-red-400">{deleteError}</p>}
            <div className="flex justify-end gap-3 pt-2">
              <Button variant="secondary" onClick={() => setDeleteTarget(null)}>{t("cancel")}</Button>
              <Button variant="danger" loading={deleteLoading} onClick={confirmDelete}>{t("adminChannelDelete")}</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
