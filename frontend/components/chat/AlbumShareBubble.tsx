"use client"

import { useTranslations } from "next-intl"

import { Link } from "@/i18n/navigation"

interface Payload {
  album_id: string
  name: string
  photo_count: number
  owner_id: string
}

interface Props {
  /** The raw `content` field of the chat message — JSON-encoded Payload. */
  content: string
  isOwn: boolean
}

/**
 * AlbumShareBubble renders a chat message of type 'album_share' as a
 * compact card with the album glyph, name, photo count, and an "Open
 * album" link. Albums have no cover photo by design, so the bubble is
 * icon-first rather than thumbnail-first.
 */
export default function AlbumShareBubble({ content, isOwn }: Props) {
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
      className={`flex w-64 items-center gap-3 rounded-xl p-3 ring-1 ${
        isOwn ? "ring-white/20 bg-white/5" : "ring-gray-700 bg-gray-950"
      }`}
    >
      <span
        aria-hidden="true"
        className="inline-flex h-12 w-12 flex-none items-center justify-center rounded-lg bg-brand-accent/15 text-brand-accent"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth={1.6}
          strokeLinecap="round"
          strokeLinejoin="round"
          className="h-6 w-6"
        >
          <rect x="3" y="3" width="18" height="18" rx="2" />
          <circle cx="9" cy="9" r="2" />
          <path d="M21 15l-5-5L5 21" />
        </svg>
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-foreground">{payload.name}</p>
        <p className="mt-0.5 flex items-center gap-1.5 text-xs text-gray-500">
          <span>{t("photoCount", { count: payload.photo_count })}</span>
          <span aria-hidden="true">·</span>
          <Link
            href={`/albums/${payload.album_id}`}
            className="font-medium text-brand-accent hover:underline"
          >
            {t("messageBubbleOpen")} →
          </Link>
        </p>
      </div>
    </div>
  )
}
