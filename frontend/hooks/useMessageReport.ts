"use client"

import { useEffect, useRef, useState } from "react"
import { useTranslations } from "next-intl"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
const NOTICE_MS = 4000

/**
 * Drives the "Report message" flow: which message is targeted, the POST to
 * /reports, localized error mapping, and a short-lived confirmation notice.
 */
export function useMessageReport(token: string | undefined) {
  const t = useTranslations("report")
  const [messageId, setMessageId] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const [sent, setSent] = useState(false)
  // Escape and backdrop clicks still reach cancel() while a request is in
  // flight; locking the target keeps that response from landing on another
  // report the user opened in the meantime.
  const pending = useRef(false)

  useEffect(() => {
    if (!sent) return
    const timer = setTimeout(() => setSent(false), NOTICE_MS)
    return () => clearTimeout(timer)
  }, [sent])

  function start(id: string) {
    if (pending.current) return
    setError("")
    setMessageId(id)
  }

  function cancel() {
    if (pending.current) return
    setMessageId(null)
    setError("")
  }

  async function submit(reason: string, description: string) {
    if (!token || !messageId || pending.current) return
    pending.current = true
    setLoading(true)
    setError("")
    try {
      const res = await fetch(`${API_URL}/reports`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ message_id: messageId, reason, description }),
      })
      if (!res.ok) {
        const data: { code?: string } = await res.json().catch(() => ({}))
        setError(
          data.code === "already_reported"
            ? t("alreadyReported")
            : data.code === "message_not_found"
            ? t("messageUnavailable")
            : t("failed"),
        )
        return
      }
      setMessageId(null)
      setSent(true)
    } catch {
      setError(t("failed"))
    } finally {
      pending.current = false
      setLoading(false)
    }
  }

  return { open: messageId !== null, loading, error, sent, start, cancel, submit }
}
