"use client"

import { useEffect, useState } from "react"
import { useTranslations } from "next-intl"

import { Link } from "@/i18n/navigation"
import Avatar from "@/components/ui/Avatar"
import Button from "@/components/ui/Button"
import Modal from "@/components/ui/Modal"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface AcceptedContact {
  contact_id: string
  user_id: string
  username: string
  email: string
  display_name: string
  avatar_url: string
}

interface Contact {
  user_id: string
  username: string
  display_name: string
  avatar_url: string
}

interface Props {
  open: boolean
  token: string
  onClose: () => void
  onCreated: (roomId: string) => void
}

export default function NewChatModal({ open, token, onClose, onCreated }: Props) {
  const t = useTranslations("chat")
  const tg = useTranslations("chatRoom")
  const tc = useTranslations("common")

  const [contacts, setContacts] = useState<Contact[]>([])
  const [loadingContacts, setLoadingContacts] = useState(true)
  const [creatingFor, setCreatingFor] = useState<string | null>(null)
  const [error, setError] = useState("")
  const [query, setQuery] = useState("")

  // The modal stays mounted while `open` toggles, so reset the contact-loading
  // state on each open transition to avoid showing a stale list during refetch.
  const [prevOpen, setPrevOpen] = useState(open)
  if (open !== prevOpen) {
    setPrevOpen(open)
    if (open) {
      setLoadingContacts(true)
      setError("")
    }
  }

  useEffect(() => {
    if (!open || !token) return
    fetch(`${API_URL}/contacts`, { headers: { Authorization: `Bearer ${token}` } })
      .then((r) => (r.ok ? r.json() : Promise.reject(r.status)))
      .then((data: AcceptedContact[]) => {
        const mapped = (Array.isArray(data) ? data : []).map((c) => ({
          user_id: c.user_id,
          username: c.username,
          display_name: c.display_name || c.email || c.username,
          avatar_url: c.avatar_url,
        }))
        // Sort alphabetically by display name (case-insensitive, locale-aware)
        // so the picker stays predictable as the contact list grows.
        mapped.sort((a, b) =>
          (a.display_name || a.username).localeCompare(b.display_name || b.username, undefined, { sensitivity: "base" })
        )
        setContacts(mapped)
        setError("")
      })
      .catch(() => setError(tg("failedLoadContacts")))
      .finally(() => setLoadingContacts(false))
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, token])

  function handleClose() {
    setQuery("")
    setError("")
    setCreatingFor(null)
    onClose()
  }

  async function handlePick(peerId: string) {
    if (creatingFor) return
    setCreatingFor(peerId)
    setError("")
    try {
      const res = await fetch(`${API_URL}/chat/rooms/dm`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({ peer_id: peerId }),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        setError((data as { error?: string }).error ?? t("newChatFailed"))
        return
      }
      const room = await res.json()
      setQuery("")
      onCreated(room.id as string)
    } catch {
      setError(tc("networkError"))
    } finally {
      setCreatingFor(null)
    }
  }

  const needle = query.trim().toLowerCase()
  const filtered = needle
    ? contacts.filter((c) => (c.display_name || c.username).toLowerCase().includes(needle))
    : contacts

  return (
    <Modal open={open} onClose={handleClose}>
      <div
        className="w-full max-w-md rounded-xl bg-gray-900 shadow-2xl ring-1 ring-gray-700"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-gray-800 px-5 py-4">
          <h2 className="text-base font-semibold text-foreground">{t("newChatTitle")}</h2>
          <button
            type="button"
            aria-label={tc("close")}
            onClick={handleClose}
            className="cursor-pointer rounded p-1 text-gray-500 transition-colors hover:text-gray-300 focus:outline-none focus:ring-2 focus:ring-brand-hover"
          >
            <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="space-y-4 p-5">
          <div className="relative">
            <svg className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-500" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
            </svg>
            <input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("newChatSearchPlaceholder")}
              aria-label={t("newChatSearchPlaceholder")}
              autoFocus
              className="w-full rounded-md border border-gray-700 bg-gray-800 py-2 pl-9 pr-4 text-base text-foreground placeholder-gray-500 focus:border-brand-hover focus:outline-none focus:ring-1 focus:ring-brand-hover"
            />
          </div>

          {!loadingContacts && contacts.length > 0 && (
            <p className="text-xs text-gray-500" aria-live="polite">
              {t("newChatCount", { count: contacts.length })}
            </p>
          )}

          {loadingContacts ? (
            <p className="text-xs text-gray-500">{tg("loadingContacts")}</p>
          ) : filtered.length === 0 ? (
            <p className="px-1 py-4 text-center text-xs text-gray-500">
              {needle ? t("newChatNoMatches") : t("newChatNoContacts")}
            </p>
          ) : (
            <ul className="max-h-72 overflow-y-auto divide-y divide-gray-800 rounded-lg ring-1 ring-gray-800">
              {filtered.map((c) => {
                const isCreating = creatingFor === c.user_id
                return (
                  <li key={c.user_id}>
                    <button
                      type="button"
                      onClick={() => handlePick(c.user_id)}
                      disabled={!!creatingFor}
                      className="flex w-full cursor-pointer items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-gray-800/60 disabled:cursor-wait disabled:opacity-60 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-brand-hover"
                    >
                      <Avatar src={c.avatar_url} name={c.display_name || "?"} size="sm" />
                      <span className="flex-1 truncate text-sm text-foreground">
                        {c.display_name || c.username}
                      </span>
                      {isCreating && (
                        <span className="h-4 w-4 animate-spin rounded-full border-2 border-gray-600 border-t-brand-hover" aria-hidden="true" />
                      )}
                    </button>
                  </li>
                )
              })}
            </ul>
          )}

          <p className="text-xs text-gray-500">
            {t("newChatFooterNote")}{" "}
            <Link
              href="/contacts"
              onClick={handleClose}
              className="text-brand-primary underline-offset-2 hover:underline focus:outline-none focus:ring-2 focus:ring-brand-hover rounded"
            >
              {t("newChatManageContacts")}
            </Link>
          </p>

          {error && <p className="text-xs text-red-400">{error}</p>}
        </div>

        <div className="flex justify-end gap-3 border-t border-gray-800 px-5 py-4">
          <Button variant="ghost" size="sm" onClick={handleClose} disabled={!!creatingFor}>
            {tc("cancel")}
          </Button>
        </div>
      </div>
    </Modal>
  )
}
