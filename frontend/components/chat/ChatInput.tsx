"use client"

import { useRef, useState } from "react"

import type { EphemeralMode } from "@/types/chat"
import { EPHEMERAL_LABELS } from "@/types/chat"

interface ChatInputProps {
  connected: boolean
  uploading: boolean
  ephemeral: EphemeralMode
  onEphemeralChange: (mode: EphemeralMode) => void
  onSend: (content: string) => void
  onAttach: (file: File) => void
  onTyping: () => void
  inputRef?: React.RefObject<HTMLInputElement | null>
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
}: ChatInputProps) {
  const [input, setInput] = useState("")
  const [showEphemeralMenu, setShowEphemeralMenu] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const typingThrottleRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const internalInputRef = useRef<HTMLInputElement>(null)
  const effectiveRef = inputRef || internalInputRef

  function handleSend() {
    const content = input.trim()
    if (!content) return
    onSend(content)
    setInput("")
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
          <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
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
          {EPHEMERAL_LABELS[ephemeral]}
        </div>
      )}

      <div className="relative flex items-center gap-3">
        <input
          ref={fileInputRef}
          type="file"
          accept="image/*,video/*,.pdf,.doc,.docx,.txt,.zip"
          className="hidden"
          onChange={handleFileChange}
        />

        {/* Attach button */}
        <button
          onClick={() => fileInputRef.current?.click()}
          disabled={!connected || uploading}
          aria-label="Attach file"
          title="Attach file"
          className="flex-none rounded-full p-2 text-gray-400 hover:bg-gray-800 hover:text-gray-200 disabled:opacity-40"
        >
          {uploading ? (
            <svg className="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
            </svg>
          ) : (
            <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13" />
            </svg>
          )}
        </button>

        {/* Ephemeral mode button */}
        <div className="relative flex-none">
          <button
            onClick={() => setShowEphemeralMenu((v) => !v)}
            disabled={!connected}
            aria-label="Ephemeral message"
            title="Ephemeral message"
            className={`rounded-full p-2 transition-colors disabled:opacity-40 ${
              ephemeral !== "off"
                ? "text-amber-400 hover:bg-amber-400/10"
                : "text-gray-400 hover:bg-gray-800 hover:text-gray-200"
            }`}
          >
            {ephemeral === "view_once" ? (
              <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                <path strokeLinecap="round" strokeLinejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
              </svg>
            ) : (
              <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                <circle cx="12" cy="12" r="9" />
                <path strokeLinecap="round" d="M12 7v5l3 3" />
              </svg>
            )}
          </button>

          {showEphemeralMenu && (
            <div className="absolute bottom-full left-0 mb-2 w-48 overflow-hidden rounded-xl border border-gray-700 bg-gray-900 shadow-xl">
              <p className="px-4 py-2 text-xs text-gray-500">Applies to messages &amp; attachments</p>
              <div className="border-t border-gray-700/60" />
              {(Object.keys(EPHEMERAL_LABELS) as EphemeralMode[]).map((mode) => (
                <button
                  key={mode}
                  onClick={() => { onEphemeralChange(mode); setShowEphemeralMenu(false) }}
                  className={`flex w-full items-center px-4 py-2.5 text-left text-sm transition-colors hover:bg-gray-800 ${
                    ephemeral === mode ? "text-amber-400" : "text-gray-300"
                  }`}
                >
                  {EPHEMERAL_LABELS[mode]}
                </button>
              ))}
            </div>
          )}
        </div>

        <label htmlFor="message-input" className="sr-only">Message</label>
        <input
          ref={effectiveRef}
          id="message-input"
          type="text"
          placeholder="Message…"
          value={input}
          onChange={(e) => {
            setInput(e.target.value)
            if (!typingThrottleRef.current) {
              onTyping()
              typingThrottleRef.current = setTimeout(() => {
                typingThrottleRef.current = null
              }, 2000)
            }
          }}
          onKeyDown={(e) => e.key === "Enter" && handleSend()}
          className="flex-1 rounded-full border border-gray-700 bg-gray-800 px-4 py-2 text-sm text-white placeholder-gray-500 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
        />
        <button
          onClick={handleSend}
          disabled={!connected || !input.trim()}
          className="rounded-full bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 disabled:opacity-40"
        >
          Send
        </button>
      </div>
    </div>
  )
}
