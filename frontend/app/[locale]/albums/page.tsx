"use client"

import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { useEffect, useState } from "react"

import AlbumCard from "@/components/albums/AlbumCard"
import CreateAlbumDialog from "@/components/albums/CreateAlbumDialog"
import Button from "@/components/ui/Button"
import Skeleton from "@/components/ui/Skeleton"
import { albumsApi, type Album } from "@/lib/albums"

type Tab = "mine" | "shared"

type LoadState =
  | { kind: "loading" }
  | { kind: "ok"; albums: Album[] }
  | { kind: "error" }

export default function AlbumsPage() {
  const { data: session } = useSession()
  const t = useTranslations("albums")
  const token = session?.accessToken
  const [tab, setTab] = useState<Tab>("mine")
  const [state, setState] = useState<LoadState>({ kind: "loading" })
  const [createOpen, setCreateOpen] = useState(false)

  useEffect(() => {
    if (!token) return
    let cancelled = false
    const fetcher = tab === "mine" ? albumsApi.listMine : albumsApi.listSharedWithMe
    fetcher(token)
      .then((rows) => {
        if (!cancelled) setState({ kind: "ok", albums: rows })
      })
      .catch(() => {
        if (!cancelled) setState({ kind: "error" })
      })
    return () => {
      cancelled = true
    }
  }, [token, tab])

  function handleCreated(a: Album) {
    if (state.kind === "ok") setState({ kind: "ok", albums: [a, ...state.albums] })
    setTab("mine")
  }

  return (
    <div className="mx-auto max-w-5xl space-y-6 px-4 py-6">
      <header className="space-y-2">
        <h1 className="text-2xl font-bold text-white">{t("title")}</h1>
        <p className="max-w-2xl text-sm text-gray-400">{t("subtitle")}</p>
      </header>

      <div className="flex flex-wrap items-center gap-3">
        <div role="tablist" aria-label={t("title")} className="flex gap-2">
          <TabButton active={tab === "mine"} onClick={() => setTab("mine")}>
            {t("tabMine")}
          </TabButton>
          <TabButton active={tab === "shared"} onClick={() => setTab("shared")}>
            {t("tabShared")}
          </TabButton>
        </div>
        <div className="ml-auto">
          <Button variant="primary" size="sm" onClick={() => setCreateOpen(true)} disabled={!token}>
            {t("create")}
          </Button>
        </div>
      </div>

      {state.kind === "error" && (
        <p role="alert" className="text-sm text-red-400">
          {t("loadError")}
        </p>
      )}

      {state.kind === "loading" && (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {Array.from({ length: 6 }, (_, i) => (
            <Skeleton key={i} className="aspect-square w-full rounded-lg" />
          ))}
        </div>
      )}

      {state.kind === "ok" && state.albums.length === 0 && (
        <div className="rounded-lg border border-dashed border-gray-800 p-10 text-center">
          <p className="text-sm text-gray-400">{tab === "mine" ? t("empty") : t("emptyShared")}</p>
          {tab === "mine" && (
            <Button variant="primary" size="sm" onClick={() => setCreateOpen(true)} className="mt-4">
              {t("emptyCta")}
            </Button>
          )}
        </div>
      )}

      {state.kind === "ok" && state.albums.length > 0 && token && (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {state.albums.map((a) => (
            <AlbumCard key={a.id} album={a} token={token} />
          ))}
        </div>
      )}

      {token && (
        <CreateAlbumDialog
          open={createOpen}
          token={token}
          onClose={() => setCreateOpen(false)}
          onCreated={handleCreated}
        />
      )}
    </div>
  )
}

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      onClick={onClick}
      className={`rounded-full px-3 py-1.5 text-xs font-medium ring-1 transition-colors ${
        active
          ? "bg-brand-accent text-white ring-brand-accent"
          : "bg-gray-900 text-gray-300 ring-gray-800 hover:bg-gray-800 hover:text-white"
      }`}
    >
      {children}
    </button>
  )
}
