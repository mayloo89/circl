"use client"

import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { useEffect, useState } from "react"

import Button from "@/components/ui/Button"
import Skeleton from "@/components/ui/Skeleton"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
const PAGE_SIZE = 50

interface ViewRecord {
  album_id: string
  album_name: string
  viewer_id: string
  viewer_name: string
  upload_id: string | null
  viewed_at: string
}

export default function AlbumViewsAdminPage() {
  const { data: session } = useSession()
  const t = useTranslations("admin")
  const accessToken = session?.accessToken

  type LoadState = { kind: "loading" } | { kind: "ok"; rows: ViewRecord[]; hasMore: boolean } | { kind: "error" }

  const [state, setState] = useState<LoadState>({ kind: "loading" })
  const [albumFilter, setAlbumFilter] = useState("")
  const [offset, setOffset] = useState(0)

  useEffect(() => {
    if (!accessToken) return
    let cancelled = false
    const qs = new URLSearchParams({ limit: String(PAGE_SIZE), offset: String(offset) })
    if (albumFilter.trim()) qs.set("album_id", albumFilter.trim())
    fetch(`${API_URL}/admin/albums/views?${qs}`, {
      headers: { Authorization: `Bearer ${accessToken}` },
    })
      .then((r) => (r.ok ? r.json() : Promise.reject(r.status)))
      .then((rows: ViewRecord[]) => {
        if (!cancelled) setState({ kind: "ok", rows, hasMore: rows.length === PAGE_SIZE })
      })
      .catch(() => { if (!cancelled) setState({ kind: "error" }) })
    return () => { cancelled = true }
  }, [accessToken, albumFilter, offset])

  function handleFilterSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setOffset(0)
  }

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <h1 className="text-xl font-semibold text-white mb-1">{t("albumViewsTitle")}</h1>
      <p className="text-sm text-gray-400 mb-6">{t("albumViewsSubtitle")}</p>

      <form onSubmit={handleFilterSubmit} className="flex gap-2 mb-6">
        <input
          type="text"
          value={albumFilter}
          onChange={(e) => setAlbumFilter(e.target.value)}
          placeholder={t("albumViewsFilterPlaceholder")}
          className="flex-1 rounded bg-gray-800 border border-gray-700 px-3 py-2 text-sm text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-brand-accent"
        />
        <Button type="submit" variant="secondary" size="sm">{t("search")}</Button>
      </form>

      {state.kind === "loading" && (
        <div className="space-y-2">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full rounded" />
          ))}
        </div>
      )}

      {state.kind === "error" && (
        <p className="text-red-400 text-sm">{t("moderationLoadError")}</p>
      )}

      {state.kind === "ok" && state.rows.length === 0 && (
        <p className="text-gray-500 text-sm">{t("albumViewsEmpty")}</p>
      )}

      {state.kind === "ok" && state.rows.length > 0 && (
        <>
          <div className="overflow-x-auto rounded-lg border border-gray-800">
            <table className="w-full text-sm">
              <thead className="bg-gray-900 text-gray-400 text-xs uppercase tracking-wide">
                <tr>
                  <th className="px-4 py-3 text-left">{t("albumViewsColAlbum")}</th>
                  <th className="px-4 py-3 text-left">{t("albumViewsColViewer")}</th>
                  <th className="px-4 py-3 text-left">{t("albumViewsColEvent")}</th>
                  <th className="px-4 py-3 text-left">{t("albumViewsColTime")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800">
                {state.rows.map((row, i) => (
                  <tr key={i} className="hover:bg-gray-900/50 transition-colors">
                    <td className="px-4 py-3 text-white font-medium">
                      <span className="block truncate max-w-[200px]" title={row.album_id}>{row.album_name}</span>
                    </td>
                    <td className="px-4 py-3 text-gray-300">
                      <span className="block truncate max-w-[160px]" title={row.viewer_id}>{row.viewer_name}</span>
                    </td>
                    <td className="px-4 py-3">
                      {row.upload_id ? (
                        <span className="inline-flex items-center rounded-full bg-brand-accent/10 px-2 py-0.5 text-xs font-medium text-brand-accent">
                          {t("albumViewsPhoto")}
                        </span>
                      ) : (
                        <span className="inline-flex items-center rounded-full bg-gray-800 px-2 py-0.5 text-xs font-medium text-gray-400">
                          {t("albumViewsListing")}
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-gray-400 whitespace-nowrap">
                      {new Date(row.viewed_at).toLocaleString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="mt-4 flex items-center gap-3">
            {offset > 0 && (
              <Button variant="secondary" size="sm" onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))}>
                ← Prev
              </Button>
            )}
            {state.hasMore && (
              <Button variant="secondary" size="sm" onClick={() => setOffset((o) => o + PAGE_SIZE)}>
                {t("albumViewsLoadMore")}
              </Button>
            )}
          </div>
        </>
      )}
    </div>
  )
}
