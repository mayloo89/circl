"use client"

import { useSession } from "next-auth/react"
import { useTranslations } from "next-intl"

import AuthedImage from "@/components/admin/AuthedImage"
import { Link } from "@/i18n/navigation"
import { absoluteAlbumURL } from "@/lib/albums"

interface Payload {
  album_id: string
  name: string
  photo_count: number
  cover_url?: string
  owner_id: string
}

interface Props {
  /** The raw `content` field of the chat message — JSON-encoded Payload. */
  content: string
  isOwn: boolean
}

/**
 * AlbumShareBubble renders a chat message of type 'album_share' as a card
 * with the album's cover, name, photo count, and an "Open album" link.
 * Content is the JSON payload the backend put in `message.content` —
 * parsed inline so the bubble is self-contained and the chat hook doesn't
 * need a type-aware decoder.
 */
export default function AlbumShareBubble({ content, isOwn }: Props) {
  const { data: session } = useSession()
  const token = session?.accessToken
  const t = useTranslations("albums")

  let payload: Payload | null = null
  try {
    payload = JSON.parse(content) as Payload
  } catch {
    payload = null
  }
  if (!payload || !payload.album_id) {
    return <span className="text-xs text-gray-500">{t("messageBubbleCaption")}</span>
  }

  return (
    <div
      className={`w-64 overflow-hidden rounded-xl ring-1 ${
        isOwn ? "ring-white/20" : "ring-gray-700"
      } bg-gray-950`}
    >
      <div className="relative h-32 w-full bg-gray-900">
        {payload.cover_url && token ? (
          <AuthedImage
            src={absoluteAlbumURL(payload.cover_url)}
            token={token}
            alt={payload.name}
            className="h-full w-full object-cover"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-gray-700">
            <svg
              aria-hidden="true"
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth={1.4}
              className="h-10 w-10"
            >
              <rect x="3" y="3" width="18" height="18" rx="2" />
              <circle cx="9" cy="9" r="2" />
              <path d="M21 15l-5-5L5 21" />
            </svg>
          </div>
        )}
      </div>
      <div className="space-y-1 p-3">
        <p className="text-[10px] uppercase tracking-wider text-gray-500">{t("messageBubbleCaption")}</p>
        <p className="truncate text-sm font-medium text-white">{payload.name}</p>
        <p className="text-xs text-gray-500">{t("photoCount", { count: payload.photo_count })}</p>
        <Link
          href={`/albums/${payload.album_id}`}
          className="mt-2 inline-block text-xs font-medium text-brand-accent hover:underline"
        >
          {t("messageBubbleOpen")} →
        </Link>
      </div>
    </div>
  )
}
