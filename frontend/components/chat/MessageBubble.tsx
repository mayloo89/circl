"use client"

import Image from "next/image"
import { useTranslations } from "next-intl"

import type { AnyMessage } from "@/types/chat"
import { formatExpiry, expiryColorClass } from "@/lib/chatHelpers"
import Avatar from "@/components/ui/Avatar"
import AlbumShareBubble from "@/components/chat/AlbumShareBubble"

interface MessageBubbleProps {
  msg: AnyMessage
  isOwn: boolean
  firstInGroup: boolean
  lastInGroup: boolean
  isLive: boolean
  isTombstone: boolean
  revealedText?: string
  isLastSeenOwn: boolean
  now: number
  onViewOnce: (id: string) => void
  onOpenMedia: (url: string, type: string) => void
}

export default function MessageBubble({
  msg,
  isOwn,
  firstInGroup,
  lastInGroup,
  isLive,
  isTombstone,
  revealedText,
  isLastSeenOwn,
  now,
  onViewOnce,
  onOpenMedia,
}: MessageBubbleProps) {
  const t = useTranslations("chatRoom")
  const avatarUrl = "sender_avatar_url" in msg ? msg.sender_avatar_url : ""
  const isViewOnce = msg.view_once
  const hasMedia =
    ("thumbnail_url" in msg && msg.thumbnail_url) ||
    (msg.type === "image" && msg.content && !isViewOnce) ||
    (msg.type === "video" && !isViewOnce) ||
    msg.type === "album_share"

  const bubbleClass = hasMedia
    ? "overflow-hidden p-0"
    : isViewOnce && isOwn
    ? "relative overflow-hidden rounded-br-sm border-2 border-dashed border-white/30 bg-gradient-to-br from-brand-primary to-purple-700 px-4 py-3 text-white"
    : msg.expires_at || isViewOnce
    ? `px-4 py-2 border-2 border-dashed ${isOwn ? "rounded-br-sm bg-brand-primary text-white border-white/30" : "rounded-bl-sm bg-gray-800 text-gray-100 border-gray-600"}`
    : `px-4 py-2 ${isOwn ? "bg-brand-primary text-white rounded-br-sm" : "bg-gray-800 text-gray-100 rounded-bl-sm"}`

  return (
    <div
      className={`flex ${isOwn ? "justify-end" : "justify-start"} ${firstInGroup ? "mt-3" : "mt-0.5"} ${isLive ? "animate-message-in" : ""}`}
    >
      {!isOwn && (
        <div className="mr-2 mt-auto flex-none self-end">
          {lastInGroup ? (
            <Avatar src={avatarUrl} name={msg.sender_name || "?"} size="sm" />
          ) : (
            <div className="h-7 w-7" />
          )}
        </div>
      )}

      <div className={`flex flex-col ${isOwn ? "items-end" : "items-start"}`}>
        {!isOwn && firstInGroup && (
          <span className="mb-1 text-xs text-gray-500">{msg.sender_name}</span>
        )}

        {isTombstone ? (
          <>
            <div className="flex items-center gap-1.5 rounded-2xl border border-dashed border-gray-700 px-3 py-1.5 text-xs text-gray-600">
              <svg className="h-3 w-3 shrink-0" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                {isViewOnce ? (
                  <path strokeLinecap="round" strokeLinejoin="round" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
                ) : (
                  <>
                    <circle cx="12" cy="12" r="9" />
                    <path strokeLinecap="round" d="M12 7v5l3 3" />
                  </>
                )}
              </svg>
              {isViewOnce ? t("viewOnceExpired") : t("messageExpired")}
            </div>
            <span className="mt-1 text-xs text-gray-700">
              {new Date(msg.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
            </span>
          </>
        ) : (
          <>
            <div className={`max-w-xs rounded-2xl text-sm ${bubbleClass}`}>
              {isViewOnce && !isOwn && !revealedText ? (
                msg.type === "image" || msg.type === "video" || msg.type === "file" ? (
                  /* View-once media — tap to reveal */
                  <button
                    onClick={() => onViewOnce(msg.id)}
                    className="flex w-44 cursor-pointer flex-col items-center gap-3 py-3 transition-opacity active:opacity-70 focus:outline-none focus:ring-2 focus:ring-brand-hover rounded-lg"
                  >
                    <div className="relative">
                      <span className="absolute inset-0 animate-ping rounded-full bg-brand-muted/30" />
                      <span className="relative flex h-12 w-12 items-center justify-center rounded-full bg-brand-hover/20">
                        {msg.type === "image" ? (
                          <svg className="h-6 w-6 text-brand-subtle" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M6.827 6.175A2.31 2.31 0 015.186 7.23c-.38.054-.757.112-1.134.175C2.999 7.58 2.25 8.507 2.25 9.574V18a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9.574c0-1.067-.75-1.994-1.802-2.169a47.865 47.865 0 00-1.134-.175 2.31 2.31 0 01-1.64-1.055l-.822-1.316a2.192 2.192 0 00-1.736-1.039 48.774 48.774 0 00-5.232 0 2.192 2.192 0 00-1.736 1.039l-.821 1.316z" />
                            <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 12.75a4.5 4.5 0 11-9 0 4.5 4.5 0 019 0zM18.75 10.5h.008v.008h-.008V10.5z" />
                          </svg>
                        ) : msg.type === "video" ? (
                          <svg className="h-6 w-6 text-brand-subtle" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 10.5l4.72-4.72a.75.75 0 011.28.53v11.38a.75.75 0 01-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 002.25-2.25v-9a2.25 2.25 0 00-2.25-2.25h-9A2.25 2.25 0 002.25 7.5v9a2.25 2.25 0 002.25 2.25z" />
                          </svg>
                        ) : (
                          <svg className="h-6 w-6 text-brand-subtle" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M18.375 12.739l-7.693 7.693a4.5 4.5 0 01-6.364-6.364l10.94-10.94A3 3 0 1119.5 7.372L8.552 18.32m.009-.01l-.01.01m5.699-9.941l-7.81 7.81a1.5 1.5 0 002.112 2.13" />
                          </svg>
                        )}
                      </span>
                    </div>
                    <span className="text-xs font-medium text-gray-300">
                      {msg.type === "image" ? t("tapToViewPhoto") : msg.type === "video" ? t("tapToViewVideo") : t("tapToOpenFile")}
                    </span>
                  </button>
                ) : (
                  /* View-once text — tap to reveal */
                  <button
                    onClick={() => onViewOnce(msg.id)}
                    className="flex cursor-pointer items-center gap-2 px-1 py-0.5 transition-opacity active:opacity-70 focus:outline-none focus:ring-2 focus:ring-brand-hover rounded"
                  >
                    <div className="relative flex-none">
                      <span className="absolute inset-0 animate-ping rounded-full bg-brand-muted/30" />
                      <span className="relative flex h-6 w-6 items-center justify-center rounded-full bg-brand-hover/20">
                        <svg className="h-3.5 w-3.5 text-brand-subtle" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
                          <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 8.25h9m-9 3H12m-9.75 1.51c0 1.6 1.123 2.994 2.707 3.227 1.129.166 2.27.293 3.423.379.35.026.67.21.865.501L12 21l2.755-4.133a1.14 1.14 0 01.865-.501 48.172 48.172 0 003.423-.379c1.584-.233 2.707-1.626 2.707-3.228V6.741c0-1.602-1.123-2.995-2.707-3.228A48.394 48.394 0 0012 3c-2.392 0-4.744.175-7.043.513C3.373 3.746 2.25 5.14 2.25 6.741v6.018z" />
                        </svg>
                      </span>
                    </div>
                    <span className="text-xs font-medium text-gray-300">{t("tapToRead")}</span>
                  </button>
                )
              ) : isViewOnce && isOwn ? (
                /* View-once sent indicator */
                <span className="flex items-center gap-2 text-sm font-medium text-white/75">
                  <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                    <path strokeLinecap="round" strokeLinejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                  </svg>
                  {msg.type === "image" ? t("photoViewOnce") : msg.type === "video" ? t("videoViewOnce") : msg.type === "file" ? t("fileViewOnce") : t("viewOnce")}
                </span>
              ) : revealedText ? (
                <span className="italic opacity-80">{revealedText}</span>
              ) : msg.type === "image" && msg.content ? (
                <button
                  onClick={() => onOpenMedia(msg.content, "image")}
                  className="block cursor-pointer overflow-hidden rounded-2xl transition-opacity active:opacity-70 focus:outline-none focus:ring-2 focus:ring-brand-hover"
                >
                  <Image
                    src={"thumbnail_url" in msg && msg.thumbnail_url ? msg.thumbnail_url : msg.content}
                    alt="Shared image"
                    width={240}
                    height={180}
                    className="max-h-60 w-auto object-cover transition-opacity hover:opacity-90"
                  />
                </button>
              ) : msg.type === "video" && msg.content ? (
                <button
                  onClick={() => onOpenMedia(msg.content, "video")}
                  className={`flex cursor-pointer items-center gap-2 px-4 py-2 transition-opacity active:opacity-70 focus:outline-none focus:ring-2 focus:ring-brand-hover rounded ${isOwn ? "text-brand-light hover:text-white" : "text-brand-muted hover:text-brand-subtle"}`}
                >
                  <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M8 5v14l11-7z" />
                  </svg>
                  <span className="underline">{t("playVideo")}</span>
                </button>
              ) : msg.type === "file" && msg.content ? (
                <a
                  href={msg.content}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={`flex items-center gap-2 ${isOwn ? "text-brand-light hover:text-white" : "text-brand-muted hover:text-brand-subtle"}`}
                >
                  <svg className="h-4 w-4 flex-none" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M18.375 12.739l-7.693 7.693a4.5 4.5 0 01-6.364-6.364l10.94-10.94A3 3 0 1119.5 7.372L8.552 18.32m.009-.01l-.01.01m5.699-9.941l-7.81 7.81a1.5 1.5 0 002.112 2.13" />
                  </svg>
                  <span className="truncate underline">{msg.content.split("/").pop() ?? "attachment"}</span>
                </a>
              ) : msg.type === "album_share" ? (
                <AlbumShareBubble content={msg.content} isOwn={isOwn} />
              ) : (
                msg.content
              )}
            </div>

            <div className="mt-1 flex items-center gap-1.5">
              {msg.expires_at && !isViewOnce && (
                <span className={`flex items-center gap-1 text-xs font-medium ${expiryColorClass(msg.expires_at, now)}`}>
                  <svg className="h-3 w-3" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                    <circle cx="12" cy="12" r="9" />
                    <path strokeLinecap="round" d="M12 7v5l3 3" />
                  </svg>
                  {formatExpiry(msg.expires_at, now)}
                </span>
              )}
              <span className="text-xs text-gray-600">
                {new Date(msg.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
              </span>
              {isOwn && (
                <svg className="h-3.5 w-3.5 text-gray-600" fill="none" stroke="currentColor" strokeWidth="2.5" viewBox="0 0 24 24" aria-hidden="true">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M4 12.5l5 5L20 6" />
                </svg>
              )}
            </div>
            {isLastSeenOwn && (
              <span className="mt-0.5 text-xs text-brand-muted">{t("seen")}</span>
            )}
          </>
        )}
      </div>
    </div>
  )
}
