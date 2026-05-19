"use client"

import { useTranslations } from "next-intl"

import AuthedImage from "@/components/admin/AuthedImage"
import { Link } from "@/i18n/navigation"
import { absoluteAlbumURL, type Album } from "@/lib/albums"

interface Props {
  album: Album
  token: string
}

/**
 * AlbumCard renders one album as a tile linking to its detail page.
 * The cover image (when set) is fetched through the same Bearer-blob
 * dance used by the admin moderation thumbnails — public URLs aren't an
 * option because the photo endpoint requires the Authorization header.
 */
export default function AlbumCard({ album, token }: Props) {
  const t = useTranslations("albums")
  return (
    <Link
      href={`/albums/${album.id}`}
      className="group relative block overflow-hidden rounded-lg bg-gray-900 ring-1 ring-gray-800 transition-colors hover:ring-brand-accent focus:outline-none focus:ring-2 focus:ring-brand-accent"
    >
      <div className="relative aspect-square w-full bg-gray-950">
        {album.cover_url ? (
          <AuthedImage
            src={absoluteAlbumURL(album.cover_url)}
            token={token}
            alt=""
            className="h-full w-full object-cover transition-transform group-hover:scale-105"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-gray-600">
            <svg
              aria-hidden="true"
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth={1.4}
              strokeLinecap="round"
              strokeLinejoin="round"
              className="h-12 w-12"
            >
              <rect x="3" y="3" width="18" height="18" rx="2" />
              <circle cx="9" cy="9" r="2" />
              <path d="M21 15l-5-5L5 21" />
            </svg>
          </div>
        )}
      </div>
      <div className="space-y-1 p-3">
        <h3 className="truncate text-sm font-medium text-white">{album.name}</h3>
        <p className="text-xs text-gray-500">{t("photoCount", { count: album.photo_count })}</p>
      </div>
    </Link>
  )
}
