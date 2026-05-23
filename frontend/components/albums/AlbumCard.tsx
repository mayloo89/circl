"use client"

import { useTranslations } from "next-intl"

import { Link } from "@/i18n/navigation"
import Avatar from "@/components/ui/Avatar"
import type { Album } from "@/lib/albums"

interface Props {
  album: Album
}

export default function AlbumCard({ album }: Props) {
  const t = useTranslations("albums")
  return (
    <Link
      href={`/albums/${album.id}`}
      className="group block overflow-hidden rounded-lg bg-gray-900 ring-1 ring-gray-800 transition-colors hover:ring-brand-accent focus:outline-none focus:ring-2 focus:ring-brand-accent"
    >
      <div className="space-y-1 p-3">
        <h3 className="truncate text-sm font-medium text-foreground">{album.name}</h3>
        <p className="text-xs text-gray-500">{t("photoCount", { count: album.photo_count })}</p>
        {album.owner_name && (
          <div className="flex items-center gap-1.5 pt-1">
            <Avatar src={album.owner_avatar_url} name={album.owner_name} size="xs" />
            <span className="truncate text-xs text-gray-400">{album.owner_name}</span>
          </div>
        )}
      </div>
    </Link>
  )
}
