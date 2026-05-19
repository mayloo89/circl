"use client"

import { useEffect, useState } from "react"
import { useTranslations } from "next-intl"

import AuthedImage from "@/components/admin/AuthedImage"
import Button from "@/components/ui/Button"
import Modal from "@/components/ui/Modal"
import Skeleton from "@/components/ui/Skeleton"
import { absoluteAlbumURL, albumsApi, type Album } from "@/lib/albums"

interface Props {
  open: boolean
  token: string
  roomID: string
  onClose: () => void
  onShared: () => void
}

/**
 * ShareAlbumDialog lets the caller pick one of their own albums and share
 * it in the current DM room. Sharing implicitly grants the recipient
 * permanent access via the chat source — that's spelled out in the hint
 * line below the list so the user knows what they're agreeing to.
 */
export default function ShareAlbumDialog({ open, token, roomID, onClose, onShared }: Props) {
  const t = useTranslations("albums")
  const [albums, setAlbums] = useState<Album[] | null>(null)
  const [submittingID, setSubmittingID] = useState<string | null>(null)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!open) return
    let cancelled = false
    albumsApi
      .listMine(token)
      .then((rows) => {
        if (!cancelled) setAlbums(rows)
      })
      .catch(() => {
        if (!cancelled) setError(t("loadError"))
      })
    return () => {
      cancelled = true
    }
  }, [open, token, t])

  async function share(album: Album) {
    setSubmittingID(album.id)
    setError("")
    try {
      await albumsApi.shareInChat(token, album.id, roomID)
      onShared()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed")
    } finally {
      setSubmittingID(null)
    }
  }

  if (!open) return null

  return (
    <Modal open onClose={onClose}>
      <div
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-md max-h-[80vh] overflow-y-auto rounded-xl bg-gray-900 p-5 shadow-2xl ring-1 ring-gray-700 space-y-4"
      >
        <div>
          <h2 className="text-lg font-semibold text-white">{t("shareInChat")}</h2>
          <p className="mt-1 text-xs text-gray-500">{t("shareInChatHint")}</p>
        </div>

        {error && (
          <p role="alert" className="text-sm text-red-400">
            {error}
          </p>
        )}

        {!albums ? (
          <div className="space-y-2">
            {Array.from({ length: 3 }, (_, i) => (
              <Skeleton key={i} className="h-16 w-full rounded" />
            ))}
          </div>
        ) : albums.length === 0 ? (
          <p className="rounded border border-dashed border-gray-800 p-6 text-center text-sm text-gray-500">
            {t("empty")}
          </p>
        ) : (
          <ul className="space-y-2">
            {albums.map((a) => (
              <li
                key={a.id}
                className="flex items-center gap-3 rounded border border-gray-800 bg-gray-950 p-2"
              >
                <div className="h-14 w-14 shrink-0 overflow-hidden rounded bg-gray-900">
                  {a.cover_url ? (
                    <AuthedImage
                      src={absoluteAlbumURL(a.cover_url)}
                      token={token}
                      alt=""
                      className="h-full w-full object-cover"
                    />
                  ) : null}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-white">{a.name}</p>
                  <p className="text-xs text-gray-500">{t("photoCount", { count: a.photo_count })}</p>
                </div>
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => share(a)}
                  loading={submittingID === a.id}
                  disabled={submittingID !== null}
                >
                  {t("shareInChat")}
                </Button>
              </li>
            ))}
          </ul>
        )}

        <div className="flex justify-end pt-2">
          <Button variant="ghost" size="sm" onClick={onClose}>
            {t("cancel")}
          </Button>
        </div>
      </div>
    </Modal>
  )
}
