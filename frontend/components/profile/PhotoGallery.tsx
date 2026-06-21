"use client"

import Image from "next/image"
import { useRef, useState } from "react"
import { useTranslations } from "next-intl"

import Button from "@/components/ui/Button"

export interface ProfilePhoto {
  id: string
  url: string
}

interface PhotoGalleryProps {
  photos: ProfilePhoto[]
  maxPhotos?: number
  editable?: boolean
  uploading?: boolean
  onAdd?: () => void
  onDelete?: (photoId: string) => void
  onReorder?: (orderedIds: string[]) => Promise<void>
  onPhotoClick?: (url: string) => void
  error?: string
}

export default function PhotoGallery({
  photos,
  maxPhotos = 6,
  editable = false,
  uploading = false,
  onAdd,
  onDelete,
  onReorder,
  onPhotoClick,
  error,
}: PhotoGalleryProps) {
  const t = useTranslations("profile")
  const tc = useTranslations("common")
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null)
  const [reordering, setReordering] = useState(false)
  const reorderingRef = useRef(false)

  if (!editable) {
    if (photos.length === 0) return null
    return (
      <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
        <h2 className="mb-4 text-lg font-semibold text-foreground">{t("photos")}</h2>
        <div className="grid grid-cols-3 gap-3">
          {photos.map((photo) => (
            <button
              key={photo.id}
              onClick={() => onPhotoClick?.(photo.url)}
              className="relative aspect-square overflow-hidden rounded-lg bg-gray-800 cursor-pointer transition-opacity hover:opacity-90 active:opacity-70 focus:outline-none focus:ring-2 focus:ring-brand-hover"
            >
              <Image
                src={photo.url}
                alt=""
                fill
                className="object-cover"
                sizes="(max-width: 512px) 33vw, 170px"
              />
            </button>
          ))}
        </div>
      </div>
    )
  }

  async function move(fromIdx: number, toIdx: number) {
    if (!onReorder || reorderingRef.current) return
    reorderingRef.current = true
    const next = [...photos]
    const [item] = next.splice(fromIdx, 1)
    next.splice(toIdx, 0, item)
    setReordering(true)
    try {
      await onReorder(next.map((p) => p.id))
    } finally {
      reorderingRef.current = false
      setReordering(false)
    }
  }

  const slots = Array.from({ length: maxPhotos }, (_, i) => photos[i] ?? null)

  return (
    <div className="rounded-lg bg-gray-900 p-6 shadow-xl ring-1 ring-gray-800">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-foreground">{t("gallery")}</h2>
        <span className="text-xs text-gray-500">{t("photosCount", { current: photos.length, max: maxPhotos })}</span>
      </div>

      <div className="grid grid-cols-3 gap-3">
        {slots.map((photo, i) => {
          if (photo) {
            const isMain = i === 0
            const isConfirming = confirmDeleteId === photo.id
            return (
              <div key={photo.id} className="relative aspect-square overflow-hidden rounded-lg bg-gray-800">
                <Image
                  src={photo.url}
                  alt="Profile photo"
                  fill
                  className="object-cover"
                  sizes="(max-width: 512px) 33vw, 170px"
                />

                {/* Main badge */}
                {isMain && (
                  <span className="absolute left-1.5 top-1.5 rounded-full bg-brand-primary/90 px-2 py-0.5 text-[10px] font-semibold text-foreground">
                    {t("mainPhoto")}
                  </span>
                )}

                {isConfirming ? (
                  <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-black/75 p-2">
                    <p className="text-center text-xs font-medium text-foreground">{t("deletePhoto")}</p>
                    <div className="flex gap-2">
                      <Button
                        variant="danger"
                        size="sm"
                        onClick={() => { onDelete?.(photo.id); setConfirmDeleteId(null) }}
                      >
                        {tc("delete")}
                      </Button>
                      <Button variant="secondary" size="sm" onClick={() => setConfirmDeleteId(null)}>{tc("cancel")}</Button>
                    </div>
                  </div>
                ) : (
                  <>
                    {/* Delete button */}
                    <button
                      aria-label={t("deletePhoto")}
                      onClick={() => setConfirmDeleteId(photo.id)}
                      className="absolute right-1.5 top-1.5 flex h-7 w-7 cursor-pointer items-center justify-center rounded-full bg-black/60 text-foreground transition-colors hover:bg-red-600 focus:outline-none focus:ring-2 focus:ring-red-500"
                    >
                      <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
                      </svg>
                    </button>

                    {/* Reorder controls — only shown when onReorder is provided */}
                    {onReorder && (
                      <div className="absolute bottom-1.5 left-0 right-0 flex items-center justify-center gap-1">
                        {/* Move back */}
                        {i > 0 && (
                          <button
                            aria-label={isMain ? undefined : i === 1 ? t("makeMainPhoto") : t("movePhotoBack")}
                            disabled={reordering}
                            onClick={() => move(i, i - 1)}
                            className="flex h-6 w-6 items-center justify-center rounded-full bg-black/70 text-foreground transition-colors hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-brand-hover disabled:opacity-50"
                          >
                            {i === 1 ? (
                              /* star / make-main icon */
                              <svg className="h-3 w-3" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                                <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
                              </svg>
                            ) : (
                              /* chevron left */
                              <svg className="h-3 w-3" fill="none" stroke="currentColor" strokeWidth="2.5" viewBox="0 0 24 24" aria-hidden="true">
                                <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
                              </svg>
                            )}
                          </button>
                        )}
                        {/* Move forward */}
                        {i < photos.length - 1 && (
                          <button
                            aria-label={t("movePhotoForward")}
                            disabled={reordering}
                            onClick={() => move(i, i + 1)}
                            className="flex h-6 w-6 items-center justify-center rounded-full bg-black/70 text-foreground transition-colors hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-brand-hover disabled:opacity-50"
                          >
                            <svg className="h-3 w-3" fill="none" stroke="currentColor" strokeWidth="2.5" viewBox="0 0 24 24" aria-hidden="true">
                              <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
                            </svg>
                          </button>
                        )}
                      </div>
                    )}
                  </>
                )}
              </div>
            )
          }

          const isUploadSlot = i === photos.length
          return (
            <div key={`empty-${i}`} className="aspect-square rounded-lg border-2 border-dashed border-gray-800">
              {isUploadSlot && (
                <button
                  type="button"
                  onClick={onAdd}
                  disabled={uploading}
                  className="flex h-full w-full cursor-pointer items-center justify-center text-gray-600 transition-colors hover:border-brand-primary hover:text-brand-hover focus:outline-none focus:ring-2 focus:ring-brand-hover"
                >
                  {uploading ? (
                    <svg className="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24" aria-hidden="true">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                    </svg>
                  ) : (
                    <svg className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                    </svg>
                  )}
                  <span className="sr-only">Add photo</span>
                </button>
              )}
            </div>
          )
        })}
      </div>

      {error && <p className="mt-3 text-sm text-red-400">{error}</p>}
    </div>
  )
}
