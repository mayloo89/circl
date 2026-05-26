"use client"

import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { useEffect, useState } from "react"

import AuthedImage from "@/components/admin/AuthedImage"
import Button from "@/components/ui/Button"
import Modal from "@/components/ui/Modal"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

interface RejectedUpload {
  upload_id: string
  user_id: string
  user_email: string
  storage_key: string
  filename: string
  content_type: string
  size_bytes: number
  code: string
  reason: string
  moderated_at: string
  score?: number
  categories?: string[]
  file_retained: boolean
}

const CODE_FILTERS = ["", "nsfw_detected", "hash_match", "size_out_of_bounds", "aspect_ratio_out_of_bounds"] as const

export default function ModerationAdminPage() {
  const { data: session } = useSession()
  const t = useTranslations("admin")
  const token = session?.accessToken
  type LoadState =
    | { kind: "idle" }
    | { kind: "ok"; items: RejectedUpload[] }
    | { kind: "error" }
  const [state, setState] = useState<LoadState>({ kind: "idle" })
  const [codeFilter, setCodeFilter] = useState<string>("")
  const [preview, setPreview] = useState<RejectedUpload | null>(null)

  // Re-fetches whenever the active token or code filter changes; state
  // transitions happen only inside the async callback so the rule
  // react-hooks/set-state-in-effect stays satisfied.
  useEffect(() => {
    if (!token) return
    let cancelled = false
    const qs = codeFilter ? `?code=${encodeURIComponent(codeFilter)}` : ""
    fetch(`${API_URL}/admin/moderation${qs}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(async (res) => {
        if (cancelled) return
        if (!res.ok) {
          setState({ kind: "error" })
          return
        }
        const data = (await res.json()) as RejectedUpload[]
        if (!cancelled) setState({ kind: "ok", items: data })
      })
      .catch(() => {
        if (!cancelled) setState({ kind: "error" })
      })
    return () => {
      cancelled = true
    }
  }, [token, codeFilter])

  const codeLabel = (code: string): string => {
    switch (code) {
      case "nsfw_detected":
        return t("moderationCodeNsfw")
      case "hash_match":
        return t("moderationCodeHash")
      case "size_out_of_bounds":
        return t("moderationCodeSize")
      case "aspect_ratio_out_of_bounds":
        return t("moderationCodeAspect")
      default:
        return code
    }
  }

  return (
    <div className="p-4 sm:p-6 space-y-5">
      <header className="space-y-1">
        <h1 className="text-xl font-semibold text-foreground">{t("moderationTitle")}</h1>
        <p className="max-w-2xl text-sm text-gray-400">{t("moderationSubtitle")}</p>
      </header>

      <div role="tablist" aria-label={t("moderation")} className="flex flex-wrap gap-2">
        {CODE_FILTERS.map((c) => {
          const active = codeFilter === c
          return (
            <button
              key={c || "all"}
              type="button"
              role="tab"
              aria-selected={active}
              onClick={() => setCodeFilter(c)}
              className={`rounded-full px-3 py-1 text-xs ring-1 transition-colors ${
                active
                  ? "bg-brand-accent text-foreground ring-brand-accent"
                  : "bg-gray-900 text-gray-300 ring-gray-800 hover:bg-gray-800 hover:text-foreground"
              }`}
            >
              {c === "" ? t("moderationCodeAll") : codeLabel(c)}
            </button>
          )
        })}
      </div>

      {state.kind === "error" && (
        <p role="alert" className="text-sm text-red-400">
          {t("moderationLoadError")}
        </p>
      )}

      {state.kind === "idle" ? (
        <div className="space-y-2">
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className="h-20 w-full rounded" />
          ))}
        </div>
      ) : null}

      {state.kind === "ok" && state.items.length === 0 ? (
        <p className="rounded border border-dashed border-gray-800 p-8 text-center text-sm text-gray-500">
          {t("moderationEmpty")}
        </p>
      ) : null}

      {state.kind === "ok" && state.items.length > 0 ? (
        <div className="overflow-x-auto rounded-lg border border-gray-800">
          <table className="min-w-full divide-y divide-gray-800 text-sm">
            <thead className="bg-gray-900 text-left text-xs font-medium uppercase tracking-wide text-gray-400">
              <tr>
                <th scope="col" className="px-4 py-3">
                  {t("moderationColPreview")}
                </th>
                <th scope="col" className="px-4 py-3">
                  {t("moderationColUser")}
                </th>
                <th scope="col" className="px-4 py-3">
                  {t("moderationColCode")}
                </th>
                <th scope="col" className="hidden lg:table-cell px-4 py-3">
                  {t("moderationColScore")}
                </th>
                <th scope="col" className="hidden lg:table-cell px-4 py-3">
                  {t("moderationColCategories")}
                </th>
                <th scope="col" className="hidden md:table-cell px-4 py-3">
                  {t("moderationColReason")}
                </th>
                <th scope="col" className="hidden sm:table-cell px-4 py-3">
                  {t("moderationColTime")}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800 bg-gray-950">
              {state.items.map((item) => (
                <tr key={item.upload_id} className="align-top">
                  <td className="px-4 py-3">
                    {item.file_retained && token ? (
                      <button
                        type="button"
                        onClick={() => setPreview(item)}
                        className="block h-16 w-16 overflow-hidden rounded ring-1 ring-gray-800 transition hover:ring-brand-accent focus:outline-none focus:ring-brand-accent"
                        aria-label={t("moderationViewFull")}
                      >
                        <AuthedImage
                          src={`${API_URL}/admin/moderation/${item.upload_id}/image`}
                          token={token}
                          alt=""
                          className="h-full w-full object-cover"
                        />
                      </button>
                    ) : (
                      <div className="flex h-16 w-16 items-center justify-center rounded bg-gray-900 text-[10px] text-gray-500">
                        {t("moderationFilePurged")}
                      </div>
                    )}
                  </td>
                  <td className="px-4 py-3 text-gray-300">
                    <div className="font-mono text-xs">{item.user_email}</div>
                    <div className="mt-0.5 text-[11px] text-gray-500">{item.filename}</div>
                  </td>
                  <td className="px-4 py-3">
                    <span className="rounded bg-gray-800 px-2 py-1 text-xs font-medium text-gray-200">
                      {codeLabel(item.code)}
                    </span>
                  </td>
                  <td className="hidden lg:table-cell px-4 py-3 font-mono text-xs tabular-nums text-gray-300">
                    {item.score != null ? item.score.toFixed(3) : "—"}
                  </td>
                  <td className="hidden lg:table-cell px-4 py-3">
                    {item.categories && item.categories.length > 0 ? (
                      <ul className="flex flex-wrap gap-1">
                        {item.categories.map((cat) => (
                          <li
                            key={cat}
                            className="rounded bg-rose-900/40 px-1.5 py-0.5 text-[11px] text-rose-200"
                          >
                            {cat}
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <span className="text-gray-600">—</span>
                    )}
                  </td>
                  <td className="hidden md:table-cell px-4 py-3 max-w-xs text-xs text-gray-400">{item.reason}</td>
                  <td className="hidden sm:table-cell px-4 py-3 text-xs text-gray-500 whitespace-nowrap">
                    {new Date(item.moderated_at).toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}

      {preview && token ? (
        <Modal open onClose={() => setPreview(null)}>
          <div
            className="max-h-[90vh] max-w-[90vw] overflow-auto rounded-xl bg-gray-900 p-4 shadow-2xl ring-1 ring-gray-700"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-3 flex items-center justify-between gap-4">
              <div className="min-w-0">
                <p className="truncate font-mono text-sm text-gray-200">{preview.filename}</p>
                <p className="text-xs text-gray-500">
                  {codeLabel(preview.code)}
                  {preview.score != null ? ` · ${preview.score.toFixed(3)}` : ""}
                </p>
              </div>
              <Button variant="ghost" size="sm" onClick={() => setPreview(null)}>
                ✕
              </Button>
            </div>
            <AuthedImage
              src={`${API_URL}/admin/moderation/${preview.upload_id}/image`}
              token={token}
              alt={preview.filename}
              className="max-h-[75vh] w-auto rounded"
            />
          </div>
        </Modal>
      ) : null}
    </div>
  )
}
