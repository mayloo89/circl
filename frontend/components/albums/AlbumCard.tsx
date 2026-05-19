"use client"

import { useTranslations } from "next-intl"

import { Link } from "@/i18n/navigation"
import type { Album } from "@/lib/albums"

interface Props {
  album: Album
}

/**
 * AlbumCard renders one album as a compact tile linking to its detail
 * page. Albums intentionally have no cover photo — the visual identity is
 * the album glyph on a brand-tinted gradient + name + count. Keeps the
 * grid uniform and avoids the "broken thumbnail" feeling when albums are
 * empty or the cover hasn't been picked.
 */
export default function AlbumCard({ album }: Props) {
  const t = useTranslations("albums")
  return (
    <Link
      href={`/albums/${album.id}`}
      className="group block overflow-hidden rounded-lg bg-gray-900 ring-1 ring-gray-800 transition-colors hover:ring-brand-accent focus:outline-none focus:ring-2 focus:ring-brand-accent"
    >
      <div className="flex aspect-square w-full items-center justify-center bg-gradient-to-br from-gray-900 via-gray-900 to-brand-accent/20">
        <svg
          aria-hidden="true"
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth={1.2}
          strokeLinecap="round"
          strokeLinejoin="round"
          className="h-12 w-12 text-brand-accent/70 transition-transform group-hover:scale-110"
        >
          <rect x="3" y="3" width="18" height="18" rx="2" />
          <circle cx="9" cy="9" r="2" />
          <path d="M21 15l-5-5L5 21" />
        </svg>
      </div>
      <div className="space-y-1 p-3">
        <h3 className="truncate text-sm font-medium text-white">{album.name}</h3>
        <p className="text-xs text-gray-500">{t("photoCount", { count: album.photo_count })}</p>
      </div>
    </Link>
  )
}
