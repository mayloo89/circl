"use client"

import { useCallback, useEffect, useState } from "react"
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
  const { data: session, status } = useSession()
  const { subscribe } = useNotificationsContext()
  const [requests, setRequests] = useState<PendingRequest[]>([])
  const [busy, setBusy] = useState<Record<string, boolean>>({})

  const load = useCallback(async () => {
    if (!session?.accessToken) return
    try {
      const res = await fetch(`${API_URL}/contacts/pending`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      })
      if (res.ok) setRequests(await res.json())
    } catch {
      // silent
    }
  }, [session?.accessToken])

  useEffect(() => {
    if (status === "authenticated") load()
  }, [status, load])

  useEffect(() => {
    return subscribe((e) => {
      if (e.type === "contact_request") load()
      if (e.type === "contact_removed") {
        const { contact_id } = e.payload as { contact_id: string }
        setRequests((prev) => prev.filter((r) => r.contact_id !== contact_id))
      }
    })
  }, [subscribe, load])

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

  if (!requests.length) return null

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
                  loading={busy[req.contact_id]}
                  onClick={() => decline(req.contact_id)}
                >
                  {t("decline")}
                </Button>
              </div>
            </li>
          ))}
        </ul>
      </div>
    </section>
  )
}
