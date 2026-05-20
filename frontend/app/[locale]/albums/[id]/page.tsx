"use client"

import { useParams } from "next/navigation"
import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"
import { useEffect, useRef, useState } from "react"

import AuthedImage from "@/components/admin/AuthedImage"
import MembersPanel from "@/components/albums/MembersPanel"
import Button from "@/components/ui/Button"
import ConfirmDialog from "@/components/ui/ConfirmDialog"
import Modal from "@/components/ui/Modal"
import Skeleton from "@/components/ui/Skeleton"
import UploadRejectionModal from "@/components/upload/UploadRejectionModal"
import { useUpload } from "@/hooks/useUpload"
import { useRouter } from "@/i18n/navigation"
import { absoluteAlbumURL, albumsApi, type Album, type AlbumPhoto, type Grant } from "@/lib/albums"

export default function AlbumDetailPage() {
  const params = useParams<{ id: string }>()
  const albumID = params?.id ?? ""
  const { data: session } = useSession()
  const router = useRouter()
  const t = useTranslations("albums")
  const tc = useTranslations("common")
  const token = session?.accessToken

  const [album, setAlbum] = useState<Album | null>(null)
  const [photos, setPhotos] = useState<AlbumPhoto[] | null>(null)
  const [viewerGrant, setViewerGrant] = useState<Grant | null>(null)
  const [loadError, setLoadError] = useState<"forbidden" | "other" | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [lightboxPhoto, setLightboxPhoto] = useState<AlbumPhoto | null>(null)

  const fileRef = useRef<HTMLInputElement>(null)
  const { upload, uploading, rejection, clearRejection, error: uploadErr } = useUpload(token)

  useEffect(() => {
    if (!token || !albumID) return
    let cancelled = false
    setLoadError(null)
    Promise.all([
      albumsApi.get(token, albumID),
      albumsApi.listPhotos(token, albumID),
      albumsApi.listGrants(token, albumID).catch(() => [] as Grant[]),
    ])
      .then(([a, p, grants]) => {
        if (cancelled) return
        setAlbum(a)
        setPhotos(p)
        if (a.role === "viewer") {
          const own = grants.find((g) => g.status === "active")
          setViewerGrant(own ?? null)
        }
      })
      .catch((err) => {
        if (cancelled) return
        const msg = err instanceof Error ? err.message : ""
        setLoadError(msg.includes("403") || msg.toLowerCase().includes("forbidden") ? "forbidden" : "other")
      })
    return () => {
      cancelled = true
    }
  }, [token, albumID])

  async function handleAddFiles(e: React.ChangeEvent<HTMLInputElement>) {
    const files = e.target.files
    if (!files || files.length === 0 || !token) return
    for (const file of Array.from(files)) {
      const result = await upload(file, "album-private")
      if (!result) continue
      try {
        await albumsApi.addPhoto(token, albumID, result.upload_id)
      } catch {
        // ignore — UI will refresh via the next listPhotos call
      }
    }
    if (fileRef.current) fileRef.current.value = ""
    const refreshed = await albumsApi.listPhotos(token, albumID).catch(() => null)
    if (refreshed) {
      setPhotos(refreshed)
      const refreshedAlbum = await albumsApi.get(token, albumID).catch(() => null)
      if (refreshedAlbum) setAlbum(refreshedAlbum)
    }
  }

  async function removePhoto(uploadID: string) {
    if (!token) return
    await albumsApi.removePhoto(token, albumID, uploadID)
    setPhotos((prev) => (prev ?? []).filter((p) => p.upload_id !== uploadID))
    if (album) setAlbum({ ...album, photo_count: Math.max(album.photo_count - 1, 0) })
  }

  async function handleDelete() {
    if (!token) return
    setDeleting(true)
    try {
      await albumsApi.remove(token, albumID)
      router.back()
    } finally {
      setDeleting(false)
    }
  }

  if (loadError === "other") {
    return (
      <div className="mx-auto max-w-2xl px-4 py-12 text-center">
        <p className="text-sm text-red-400">{t("loadError")}</p>
      </div>
    )
  }

  if (!album || photos === null) {
    return (
      <>
        <div className="mx-auto max-w-5xl space-y-4 px-4 py-6">
          <Skeleton className="h-8 w-64" />
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
            {Array.from({ length: 8 }, (_, i) => (
              <Skeleton key={i} className="aspect-square w-full rounded-lg" />
            ))}
          </div>
        </div>
        {loadError === "forbidden" && (
          <Modal open onClose={() => router.back()}>
            <div
              className="w-full max-w-sm rounded-xl bg-gray-900 p-6 shadow-2xl ring-1 ring-gray-700"
              onClick={(e) => e.stopPropagation()}
            >
              <h2 className="text-lg font-semibold text-white">{t("noAccess")}</h2>
              <p className="mt-2 text-sm text-gray-400">{t("noAccessHint")}</p>
              <div className="mt-5 flex justify-end">
                <Button variant="ghost" size="sm" onClick={() => router.back()}>
                  {tc("back")}
                </Button>
              </div>
            </div>
          </Modal>
        )}
      </>
    )
  }

  const isOwner = album.role === "owner"

  return (
    <div className="mx-auto max-w-5xl space-y-6 px-4 py-6">
      {!isOwner && (
        <button
          type="button"
          onClick={() => router.back()}
          className="inline-flex items-center gap-1.5 text-sm text-gray-400 hover:text-white transition-colors"
        >
          <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
          </svg>
          {tc("back")}
        </button>
      )}
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <h1 className="truncate text-2xl font-bold text-white">{album.name}</h1>
          {album.description && <p className="mt-1 max-w-2xl text-sm text-gray-400">{album.description}</p>}
          <p className="mt-1 text-xs text-gray-500">{t("photoCount", { count: album.photo_count })}</p>
        </div>
        <div className="flex gap-2">
          {isOwner && (
            <>
              <Button variant="primary" size="sm" onClick={() => fileRef.current?.click()} loading={uploading}>
                {t("addPhoto")}
              </Button>
              <Button variant="danger" size="sm" onClick={() => setConfirmDelete(true)}>
                {t("delete")}
              </Button>
            </>
          )}
        </div>
      </header>

      <p
        className={`rounded border px-3 py-2 text-xs ${
          isOwner
            ? "border-gray-800 bg-gray-900/40 text-gray-400"
            : "border-amber-900/40 bg-amber-900/10 text-amber-300"
        }`}
        role="note"
      >
        {isOwner
          ? t("ownerNotice")
          : viewerGrant?.expires_at
          ? t("viewerNoticeExpiry", { date: new Date(viewerGrant.expires_at).toLocaleDateString() })
          : t("viewerNotice")}
      </p>

      {uploadErr && (
        <p role="alert" className="text-sm text-red-400">
          {uploadErr}
        </p>
      )}

      <input
        ref={fileRef}
        type="file"
        accept="image/jpeg,image/png,image/webp"
        multiple
        onChange={handleAddFiles}
        className="hidden"
      />

      {photos.length === 0 ? (
        <div className="rounded-lg border border-dashed border-gray-800 p-10 text-center">
          <p className="text-sm text-gray-500">{t("empty")}</p>
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {photos.map((p) => (
            <div
              key={p.upload_id}
              className="group relative aspect-square overflow-hidden rounded-lg bg-gray-900 ring-1 ring-gray-800"
            >
              {/* Full-area button opens the lightbox — no nested buttons */}
              <button
                type="button"
                onClick={() => setLightboxPhoto(p)}
                aria-label={t("viewPhoto")}
                className="absolute inset-0 h-full w-full cursor-zoom-in focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-primary focus-visible:ring-inset"
              >
                <AuthedImage
                  key={p.upload_id}
                  src={absoluteAlbumURL(p.url)}
                  token={token!}
                  alt={p.filename}
                  className="h-full w-full object-cover"
                />
              </button>
              {isOwner && (
                <button
                  type="button"
                  onClick={() => removePhoto(p.upload_id)}
                  aria-label={t("removePhoto")}
                  className="absolute right-1.5 top-1.5 z-10 inline-flex h-8 w-8 items-center justify-center rounded-full bg-black/70 text-white opacity-0 transition-opacity hover:bg-rose-700 group-hover:opacity-100 focus:opacity-100"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth={2}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="h-4 w-4"
                    aria-hidden="true"
                  >
                    <path d="M3 6h18" />
                    <path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                    <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
                  </svg>
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {isOwner && token && <MembersPanel albumID={albumID} token={token} />}

      {lightboxPhoto && (
        <Modal open onClose={() => setLightboxPhoto(null)}>
          <button
            type="button"
            aria-label={tc("close")}
            onClick={() => setLightboxPhoto(null)}
            className="absolute right-4 top-4 z-10 cursor-pointer rounded-full p-2 text-white/70 transition-colors hover:text-white focus:outline-none focus:ring-2 focus:ring-white/50"
          >
            <svg className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
          <AuthedImage
            key={lightboxPhoto.upload_id}
            src={absoluteAlbumURL(lightboxPhoto.url)}
            token={token!}
            alt={lightboxPhoto.filename}
            className="max-h-[90vh] max-w-[90vw] rounded-lg object-contain"
          />
        </Modal>
      )}

      <UploadRejectionModal rejection={rejection} onClose={clearRejection} />

      <ConfirmDialog
        open={confirmDelete}
        title={t("deleteConfirmTitle")}
        message={t("deleteConfirmBody")}
        confirmLabel={t("delete")}
        loading={deleting}
        onConfirm={handleDelete}
        onCancel={() => setConfirmDelete(false)}
      />
    </div>
  )
}
