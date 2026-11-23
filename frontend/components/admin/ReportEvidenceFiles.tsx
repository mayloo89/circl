"use client"

import { useEffect, useState } from "react"
import { useTranslations } from "next-intl"

import AuthedImage from "@/components/admin/AuthedImage"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export interface EvidenceFile {
  id: string
  report_id: string
  kind: "message_media" | "profile_photo"
  content_type: string
  size_bytes: number
  sha256: string
  created_at: string
}

type State =
  | { kind: "loading" }
  | { kind: "error" }
  | { kind: "ok"; files: EvidenceFile[] }

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

/**
 * Media preserved with a report. Files stay hidden until a moderator reveals
 * one, so opening a report never shows abusive imagery unasked. CSAM copies
 * are kept for law enforcement and never viewable: only their hash shows.
 */
export default function ReportEvidenceFiles({
  reportId,
  restricted,
  token,
}: {
  reportId: string
  restricted: boolean
  token: string
}) {
  const t = useTranslations("admin")
  const [state, setState] = useState<State>({ kind: "loading" })
  const [revealed, setRevealed] = useState<ReadonlySet<string>>(new Set())

  useEffect(() => {
    let cancelled = false
    fetch(`${API_URL}/reports/${reportId}/evidence`, { headers: { Authorization: `Bearer ${token}` } })
      .then(async (res) => {
        if (!res.ok) throw new Error(String(res.status))
        const files: EvidenceFile[] = await res.json()
        if (!cancelled) setState({ kind: "ok", files })
      })
      .catch(() => {
        if (!cancelled) setState({ kind: "error" })
      })
    return () => {
      cancelled = true
    }
  }, [reportId, token])

  if (state.kind === "loading") {
    return <Skeleton className="h-16 w-full" />
  }
  if (state.kind === "error") {
    return <p className="text-sm text-red-400">{t("evidenceLoadFailed")}</p>
  }
  if (state.files.length === 0) return null

  return (
    <section aria-labelledby={`evidence-${reportId}`} className="space-y-2 rounded border border-gray-700 bg-gray-950 p-3">
      <h3 id={`evidence-${reportId}`} className="text-xs font-medium text-gray-300">
        {t("evidenceTitle", { count: state.files.length })}
      </h3>
      {restricted && (
        <p className="rounded bg-rose-950 px-3 py-2 text-xs text-rose-200 ring-1 ring-rose-800">{t("evidenceRestricted")}</p>
      )}
      <ul className="space-y-3">
        {state.files.map((file) => {
          const label = file.kind === "profile_photo" ? t("evidenceKindProfilePhoto") : t("evidenceKindMessageMedia")
          const canReveal = !restricted && file.content_type.startsWith("image/")
          return (
            <li key={file.id} className="space-y-2 text-xs text-gray-400">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span>
                  <span className="font-medium text-gray-200">{label}</span> · {file.content_type} · {formatSize(file.size_bytes)}
                </span>
                {canReveal && !revealed.has(file.id) && (
                  <button
                    type="button"
                    onClick={() => setRevealed((prev) => new Set(prev).add(file.id))}
                    className="cursor-pointer rounded bg-gray-800 px-3 py-1.5 font-medium text-gray-200 hover:bg-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-hover"
                  >
                    {t("evidenceReveal")}
                  </button>
                )}
              </div>
              <p className="break-all font-mono text-[11px] text-gray-500">
                <span className="sr-only">{t("evidenceHashLabel")} </span>sha256:{file.sha256}
              </p>
              {canReveal && revealed.has(file.id) && (
                <AuthedImage
                  key={file.id}
                  src={`${API_URL}/reports/${reportId}/evidence/${file.id}`}
                  token={token}
                  alt={label}
                  className="max-h-80 w-auto rounded"
                />
              )}
            </li>
          )
        })}
      </ul>
    </section>
  )
}
