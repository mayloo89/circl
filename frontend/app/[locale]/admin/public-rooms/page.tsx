"use client"

import { useSession } from "next-auth/react"
import { useCallback, useEffect, useState } from "react"
import { useLocale, useTranslations } from "next-intl"
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
  const t = useTranslations("admin")
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
        setError(res.status === 409 ? t("createPublicRoomDuplicate") : text.trim() || t("createPublicRoomFailed"))
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
        <h2 className="text-base font-semibold text-foreground">{t("adminPublicRoomCreateTitle")}</h2>
        <p className="text-xs text-gray-500">
          {t("adminPublicRoomCreateNote")}
        </p>
        {error && <p className="text-sm text-red-400">{error}</p>}
        <div>
          <label className="block text-xs text-gray-400 mb-1">{t("adminChannelNameLabel")}</label>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("adminPublicRoomNamePlaceholder")}
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
          <Button variant="primary" loading={loading} disabled={!name.trim()} onClick={submit}>{t("create")}</Button>
        </div>
      </div>
    </div>
  )
}

export default function AdminPublicRoomsPage() {
  const { data: session } = useSession()
  const t = useTranslations("admin")
  const locale = useLocale()
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
      setError(t("loadAdminPublicRoomsFailed"))
    } finally {
      setLoading(false)
    }
  }, [session, t])

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
        setDeleteError(text.trim() || t("deletePublicRoomFailed"))
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
        <h1 className="text-2xl font-bold text-foreground">{t("adminPublicRoomsTitle")}</h1>
        <Button variant="primary" size="sm" onClick={() => setCreateOpen(true)}>
          {t("adminPublicRoomCreate")}
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
                  <td className="px-4 py-3"><Skeleton className="h-4 w-16 ml-auto" /></td>
                </tr>
              ))
            ) : rooms.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-4 py-8 text-center text-gray-500 bg-gray-950">
                  {t("adminPublicRoomNoFound")}
                </td>
              </tr>
            ) : (
              rooms.map((room) => (
                <tr key={room.id} className="bg-gray-950 hover:bg-gray-900">
                  <td className="px-4 py-3 font-medium text-gray-100">
                    <span className="inline-flex items-center gap-2">
                      {room.name}
                      <span className="rounded-full bg-emerald-500/10 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-emerald-400">
                        {t("adminPublicRoomPublicBadge")}
                      </span>
                    </span>
                  </td>
                  <td className="hidden md:table-cell px-4 py-3 text-gray-400 max-w-xs truncate">
                    {room.description || <span className="text-gray-600 italic">{t("adminPublicRoomNoDescription")}</span>}
                  </td>
                  <td className="hidden sm:table-cell px-4 py-3 text-gray-400">
                    {new Date(room.created_at).toLocaleDateString(locale)}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <Button
                      size="sm"
                      variant="danger"
                      onClick={() => { setDeleteTarget(room); setDeleteError("") }}
                    >
                      {t("adminChannelDelete")}
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
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/80">
          <div className="w-full max-w-sm rounded-lg bg-gray-900 ring-1 ring-gray-700 p-6 space-y-4">
            <h2 className="text-base font-semibold text-foreground">{t("adminPublicRoomDeleteTitle", { name: deleteTarget.name })}</h2>
            <p className="text-sm text-gray-400">
              {t("adminPublicRoomDeleteWarning")}
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
