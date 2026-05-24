"use client"

import { useEffect, useState } from "react"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { useNotificationsContext } from "@/contexts/NotificationsContext"
import Avatar from "@/components/ui/Avatar"
import Button from "@/components/ui/Button"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface PendingRequest {
  contact_id: string
  user_id: string
  display_name: string
  avatar_url: string
}

export default function PendingRequestsWidget() {
  const t = useTranslations("home")
  const tc = useTranslations("common")
  const { data: session, status } = useSession()
  const { subscribe } = useNotificationsContext()
  const [requests, setRequests] = useState<PendingRequest[]>([])
  const [busy, setBusy] = useState<Record<string, boolean>>({})
  const [confirming, setConfirming] = useState<string | null>(null)
  const [retryKey, setRetryKey] = useState(0)
  const [errorKey, setErrorKey] = useState<number | null>(null)

  const error = errorKey === retryKey

  useEffect(() => {
    if (status !== "authenticated" || !session?.accessToken) return
    const token = session.accessToken
    let cancelled = false
    fetch(`${API_URL}/contacts/pending`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => (res.ok ? res.json() : Promise.reject()))
      .then((data: PendingRequest[]) => {
        if (!cancelled) setRequests(Array.isArray(data) ? data : [])
      })
      .catch(() => {
        if (!cancelled) setErrorKey(retryKey)
      })
    return () => { cancelled = true }
  }, [status, session?.accessToken, retryKey])

  useEffect(() => {
    return subscribe((e) => {
      if (e.type === "contact_request") setRetryKey((k) => k + 1)
      if (e.type === "contact_removed") {
        const { contact_id } = e.payload as { contact_id: string }
        setRequests((prev) => prev.filter((r) => r.contact_id !== contact_id))
      }
    })
  }, [subscribe])

  async function accept(contactID: string) {
    if (!session?.accessToken) return
    setBusy((b) => ({ ...b, [contactID]: true }))
    try {
      const res = await fetch(`${API_URL}/contacts/${contactID}/accept`, {
        method: "PUT",
        headers: { Authorization: `Bearer ${session.accessToken}` },
      })
      if (res.ok) setRequests((prev) => prev.filter((r) => r.contact_id !== contactID))
    } catch {
      // silent
    } finally {
      setBusy((b) => ({ ...b, [contactID]: false }))
    }
  }

  async function decline(contactID: string) {
    if (!session?.accessToken) return
    setBusy((b) => ({ ...b, [contactID]: true }))
    try {
      const res = await fetch(`${API_URL}/contacts/${contactID}`, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${session.accessToken}` },
      })
      if (res.ok) setRequests((prev) => prev.filter((r) => r.contact_id !== contactID))
    } catch {
      // silent
    } finally {
      setBusy((b) => ({ ...b, [contactID]: false }))
    }
  }

  if (!requests.length && !error) return null

  return (
    <section aria-labelledby="pending-heading">
      <div className="rounded-card bg-gray-900 dark:bg-white/[0.04] shadow-card ring-1 ring-brand-primary/20 dark:ring-white/[0.08] overflow-hidden">
        <div className="px-4 py-3">
          <h2 id="pending-heading" className="flex items-center gap-2 text-sm font-semibold text-foreground">
            <svg aria-hidden="true" className="h-4 w-4 flex-none text-brand-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
              <path strokeLinecap="round" strokeLinejoin="round" d="M18 9v3m0 0v3m0-3h3m-3 0h-3m-2-5a4 4 0 11-8 0 4 4 0 018 0zM3 20a6 6 0 0112 0v1H3v-1z" />
            </svg>
            {t("pendingRequests")}
          </h2>
        </div>
        {error && !requests.length ? (
          <div className="flex items-center gap-3 px-4 pb-4 pt-1">
            <p className="text-sm text-gray-500">{tc("loadFailed")}</p>
            <button
              onClick={() => setRetryKey((k) => k + 1)}
              className="text-xs text-brand-subtle hover:text-brand-primary transition-colors"
            >
              {tc("retry")}
            </button>
          </div>
        ) : (
        <ul>
          {requests.map((req) => (
            <li
              key={req.contact_id}
              className="flex items-center gap-3 px-4 py-3 border-t border-gray-800 dark:border-white/[0.06]"
            >
              <Avatar src={req.avatar_url} name={req.display_name || "?"} size="md" />
              <span className="flex-1 min-w-0 text-sm font-medium text-foreground truncate">
                {req.display_name}
              </span>
              {confirming === req.contact_id ? (
                <div className="flex items-center gap-2 flex-none">
                  <span className="text-xs text-gray-400">{tc("confirm")}?</span>
                  <Button
                    variant="danger"
                    size="sm"
                    loading={busy[req.contact_id]}
                    onClick={() => { setConfirming(null); decline(req.contact_id) }}
                  >
                    {t("decline")}
                  </Button>
                  <button
                    onClick={() => setConfirming(null)}
                    className="text-xs text-gray-400 hover:text-gray-200 transition-colors"
                  >
                    {tc("cancel")}
                  </button>
                </div>
              ) : (
                <div className="flex gap-2 flex-none">
                  <Button
                    variant="accent"
                    size="sm"
                    loading={busy[req.contact_id]}
                    onClick={() => accept(req.contact_id)}
                  >
                    {t("accept")}
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => setConfirming(req.contact_id)}
                  >
                    {t("decline")}
                  </Button>
                </div>
              )}
            </li>
          ))}
        </ul>
        )}
      </div>
    </section>
  )
}
