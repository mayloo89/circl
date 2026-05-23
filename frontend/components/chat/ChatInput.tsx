"use client"

import { useRef, useState } from "react"
import { useTranslations } from "next-intl"

import type { EphemeralMode } from "@/types/chat"
import { EPHEMERAL_LABELS } from "@/types/chat"
import { useMenuKeyboard } from "@/hooks/useMenuKeyboard"

interface ChatInputProps {
  connected: boolean
  uploading: boolean
  ephemeral: EphemeralMode
  onEphemeralChange: (mode: EphemeralMode) => void
  onSend: (content: string) => void
  onAttach: (file: File) => void
  onTyping: () => void
  inputRef?: React.RefObject<HTMLTextAreaElement | null>
  disableAttach?: boolean
  disableEphemeral?: boolean
  /** Optional handler — when set, a "share album" icon button appears next
   * to the file attach button. RoomView only wires this for DM rooms. */
  onShareAlbum?: () => void
}

export default function ChatInput({
  connected,
  uploading,
  ephemeral,
  onEphemeralChange,
  onSend,
  onAttach,
  onTyping,
  inputRef,
  disableAttach = false,
  disableEphemeral = false,
  onShareAlbum,
}: ChatInputProps) {
  const t = useTranslations("chatRoom")
  const ephemeralLabels: Record<EphemeralMode, string> = {
    off: t("ephemeralOff"),
    view_once: t("ephemeralViewOnce"),
    "15m": t("ephemeral15m"),
    "30m": t("ephemeral30m"),
    "1h": t("ephemeral1h"),
    "6h": t("ephemeral6h"),
    "12h": t("ephemeral12h"),
    "24h": t("ephemeral24h"),
  }
  const [input, setInput] = useState("")
  const [showEphemeralMenu, setShowEphemeralMenu] = useState(false)
  const [showAttachMenu, setShowAttachMenu] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const typingThrottleRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const internalInputRef = useRef<HTMLTextAreaElement>(null)
  const effectiveRef = inputRef ?? internalInputRef
  const ephemeralMenuRef = useRef<HTMLDivElement>(null)
  const attachMenuRef = useRef<HTMLDivElement>(null)
  useMenuKeyboard({ open: showEphemeralMenu, containerRef: ephemeralMenuRef, onClose: () => setShowEphemeralMenu(false) })
  useMenuKeyboard({ open: showAttachMenu, containerRef: attachMenuRef, onClose: () => setShowAttachMenu(false) })

  function autoResize(el: HTMLTextAreaElement) {
    el.style.height = "auto"
    el.style.height = el.scrollHeight + "px"
  }

  function handleSend() {
    const content = input.trim()
    if (!content) return
    onSend(content)
    setInput("")
    const el = effectiveRef.current
    if (el) el.style.height = "auto"
  }

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    e.target.value = ""
    onAttach(file)
  }

  return (
    <div className="border-t border-gray-800 bg-gray-900 px-4 py-3">
      {ephemeral !== "off" && (
        <div className="mb-2 flex items-center gap-1.5 text-xs text-amber-400">
          <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
            {ephemeral === "view_once" ? (
              <>
                <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                <path strokeLinecap="round" strokeLinejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
              </>
            ) : (
              <>
                <circle cx="12" cy="12" r="9" />
                <path strokeLinecap="round" d="M12 7v5l3 3" />
              </>
            )}
          </svg>
          {ephemeralLabels[ephemeral]}
        </div>
      )}

      <div className="relative flex items-end gap-3">
        {!disableAttach && (
          <>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*,video/*"
              capture="environment"
              className="hidden"
              onChange={handleFileChange}
            />
            <div className="relative flex-none">
              <button
                type="button"
                onClick={() => {
                  if (onShareAlbum) {
                    setShowAttachMenu((v) => !v)
                  } else {
                    fileInputRef.current?.click()
                  }
                }}
                disabled={!connected || uploading}
                aria-label={t("attachFile")}
                aria-haspopup={onShareAlbum ? "menu" : undefined}
                aria-expanded={onShareAlbum ? showAttachMenu : undefined}
                title={t("attachFile")}
                className="cursor-pointer rounded-full p-2 text-gray-400 hover:bg-gray-800 hover:text-gray-200 disabled:opacity-40 focus:outline-none focus:ring-2 focus:ring-brand-hover"
              >
                {uploading ? (
                  <svg className="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24" aria-hidden="true">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
                  </svg>
                ) : (
                  <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13" />
                  </svg>
                )}
              </button>

              {onShareAlbum && showAttachMenu && (
                <div
                  ref={attachMenuRef}
                  role="menu"
                  aria-label={t("attachFile")}
                  className="absolute bottom-full left-0 mb-2 w-48 overflow-hidden rounded-xl border border-gray-700 bg-gray-900 shadow-xl"
                >
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => { fileInputRef.current?.click(); setShowAttachMenu(false) }}
                    className="flex w-full cursor-pointer items-center gap-3 px-4 py-2.5 text-left text-sm text-gray-300 transition-colors hover:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-brand-hover"
                  >
                    <svg className="h-4 w-4 flex-none text-gray-400" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13" />
                    </svg>
                    {t("attachFile")}
                  </button>
                  <div className="border-t border-gray-700/60" />
                  <button
                    type="button"
                    role="menuitem"
                    onClick={() => { onShareAlbum(); setShowAttachMenu(false) }}
                    className="flex w-full cursor-pointer items-center gap-3 px-4 py-2.5 text-left text-sm text-gray-300 transition-colors hover:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-brand-hover"
                  >
                    <svg className="h-4 w-4 flex-none text-gray-400" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                      <rect x="3" y="3" width="18" height="18" rx="2" />
                      <circle cx="9" cy="9" r="2" />
                      <path d="M21 15l-5-5L5 21" />
                    </svg>
                    {t("shareAlbum")}
                  </button>
                </div>
              )}
            </div>
          </>
        )}

        {/* Ephemeral mode button */}
        {!disableEphemeral && <div className="relative flex-none">
          <button
            type="button"
            onClick={() => setShowEphemeralMenu((v) => !v)}
            disabled={!connected}
            aria-label={t("ephemeralMessage")}
            aria-haspopup="menu"
            aria-expanded={showEphemeralMenu}
            title={t("ephemeralMessage")}
            className={`cursor-pointer rounded-full p-2 transition-colors disabled:opacity-40 focus:outline-none focus:ring-2 focus:ring-brand-hover ${
              ephemeral !== "off"
                ? "text-amber-400 hover:bg-amber-400/10"
                : "text-gray-400 hover:bg-gray-800 hover:text-gray-200"
            }`}
          >
            {ephemeral === "view_once" ? (
              <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                <path strokeLinecap="round" strokeLinejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
              </svg>
            ) : (
              <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24" aria-hidden="true">
                <circle cx="12" cy="12" r="9" />
                <path strokeLinecap="round" d="M12 7v5l3 3" />
              </svg>
            )}
          </button>

          {showEphemeralMenu && (
            <div
              ref={ephemeralMenuRef}
              role="menu"
              aria-label={t("ephemeralMessage")}
              className="absolute bottom-full left-0 mb-2 w-48 overflow-hidden rounded-xl border border-gray-700 bg-gray-900 shadow-xl"
            >
              <p className="px-4 py-2 text-xs text-gray-500">{t("ephemeralAppliesTo")}</p>
              <div className="border-t border-gray-700/60" />
              {(Object.keys(EPHEMERAL_LABELS) as EphemeralMode[]).map((mode) => (
                <button
                  key={mode}
                  type="button"
                  role="menuitem"
                  onClick={() => { onEphemeralChange(mode); setShowEphemeralMenu(false) }}
                  className={`flex w-full cursor-pointer items-center px-4 py-2.5 text-left text-sm transition-colors hover:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-brand-hover ${
                    ephemeral === mode ? "text-amber-400" : "text-gray-300"
                  }`}
                >
                  {ephemeralLabels[mode]}
                </button>
              ))}
            </div>
          )}
        </div>}

        <label htmlFor="message-input" className="sr-only">Message</label>
        <textarea
          ref={effectiveRef}
          id="message-input"
          rows={1}
          placeholder={t("messagePlaceholder")}
          value={input}
          onChange={(e) => {
            setInput(e.target.value)
            autoResize(e.target)
            if (!typingThrottleRef.current) {
              onTyping()
              typingThrottleRef.current = setTimeout(() => {
                typingThrottleRef.current = null
              }, 2000)
            }
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault()
              handleSend()
            }
          }}
          className="flex-1 max-h-40 resize-none overflow-y-auto rounded-2xl border border-gray-700 bg-gray-800 px-4 py-2 text-base text-foreground placeholder-gray-500 focus:border-brand-hover focus:outline-none focus:ring-1 focus:ring-brand-hover"
        />
        <button
          onClick={handleSend}
          disabled={!connected || !input.trim()}
          className="cursor-pointer rounded-full bg-brand-primary px-4 py-2 text-sm font-medium text-white hover:bg-brand-hover disabled:opacity-40 focus:outline-none focus:ring-2 focus:ring-brand-hover"
        >
          {t("send")}
        </button>
      </div>
    </div>
  )
}
