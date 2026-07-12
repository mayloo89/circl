"use client"

import { useEffect, useState } from "react"
import { useTranslations } from "next-intl"
import Avatar from "@/components/ui/Avatar"
import Button from "@/components/ui/Button"
import Input from "@/components/ui/Input"
import Modal from "@/components/ui/Modal"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface Contact {
  user_id: string
  username: string
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

interface Props {
  open: boolean
  token: string
  onClose: () => void
  onCreated: (roomId: string) => void
}

export default function CreateGroupModal({ open, token, onClose, onCreated }: Props) {
  const t = useTranslations("chatRoom")
  const tc = useTranslations("common")
  const [name, setName] = useState("")
  const [contacts, setContacts] = useState<Contact[]>([])
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [loading, setLoading] = useState(false)
  const [loadingContacts, setLoadingContacts] = useState(true)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!open || !token) return
    fetch(`${API_URL}/contacts`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => (r.ok ? r.json() : Promise.reject(r.status)))
      .then((data: AcceptedContact[]) => {
        setContacts(
          (Array.isArray(data) ? data : []).map((c) => ({
            user_id: c.user_id,
            username: c.username,
            display_name: c.display_name || c.email || c.username,
            avatar_url: c.avatar_url,
          }))
        )
        setError("")
      })
      .catch(() => setError(t("failedLoadContacts")))
      .finally(() => setLoadingContacts(false))
  }, [open, token, t])

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) { next.delete(id) } else { next.add(id) }
      return next
    })
  }

  async function handleCreate() {
    if (!name.trim()) { setError(t("groupNameRequired")); return }
    setLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/chat/rooms`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({ name: name.trim(), member_ids: [...selected] }),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        setError((data as { error?: string }).error ?? t("failedCreateGroup"))
        return
      }
      const room = await res.json()
      setName("")
      setSelected(new Set())
      onCreated(room.id as string)
    } catch {
      setError(tc("networkError"))
    } finally {
      setLoading(false)
    }
  }

  function handleClose() {
    setName("")
    setSelected(new Set())
    setError("")
    onClose()
  }

  return (
    <Modal open={open} onClose={handleClose}>
      <div
        className="w-full max-w-md rounded-xl bg-gray-900 shadow-2xl ring-1 ring-gray-700"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-gray-800 px-5 py-4">
          <h2 className="text-base font-semibold text-foreground">{t("createGroupTitle")}</h2>
          <button
            type="button"
            aria-label={tc("close")}
            onClick={handleClose}
            className="text-gray-500 hover:text-gray-300"
          >
            ✕
          </button>
        </div>

        <div className="space-y-4 p-5">
          <Input
            label={t("groupNameLabel")}
            placeholder={t("createGroupNamePlaceholder")}
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus
          />

          <div>
            <p className="mb-2 text-xs font-medium text-gray-400">
              {selected.size > 0 ? t("addContactsSelected", { count: selected.size }) : t("addContacts")}
            </p>
            {loadingContacts ? (
              <p className="text-xs text-gray-500">{t("loadingContacts")}</p>
            ) : contacts.length === 0 ? (
              <p className="text-xs text-gray-500">{t("noContactsYet")}</p>
            ) : (
              <ul className="max-h-60 overflow-y-auto divide-y divide-gray-800 rounded-lg ring-1 ring-gray-800">
                {contacts.map((c) => {
                  const checked = selected.has(c.user_id)
                  return (
                    <li key={c.user_id}>
                      <button
                        type="button"
                        onClick={() => toggle(c.user_id)}
                        className={`flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-gray-800/60 ${checked ? "bg-brand-deep/40" : ""}`}
                      >
                        <Avatar src={c.avatar_url} name={c.display_name || "?"} size="sm" />
                        <span className="flex-1 truncate text-sm text-foreground">
                          {c.display_name || c.username}
                        </span>
                        <span
                          className={`flex h-5 w-5 items-center justify-center rounded-full border text-xs font-bold transition-colors ${
                            checked
                              ? "border-brand-hover bg-brand-primary text-foreground"
                              : "border-gray-600 text-transparent"
                          }`}
                          aria-hidden="true"
                        >
                          ✓
                        </span>
                      </button>
                    </li>
                  )
                })}
              </ul>
            )}
          </div>

          {error && <p className="text-xs text-red-400">{error}</p>}
        </div>

        <div className="flex justify-end gap-3 border-t border-gray-800 px-5 py-4">
          <Button variant="ghost" size="sm" onClick={handleClose} disabled={loading}>
            {tc("cancel")}
          </Button>
          <Button
            variant="primary"
            size="sm"
            onClick={handleCreate}
            loading={loading}
            disabled={!name.trim()}
          >
            {t("createGroup")}
          </Button>
        </div>
      </div>
    </Modal>
  )
}
