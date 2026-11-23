"use client"

import { useEffect, useId, useRef, useState } from "react"
import { useTranslations } from "next-intl"

import { useMenuKeyboard } from "@/hooks/useMenuKeyboard"

interface MessageActionsMenuProps {
  onReport: () => void
  /** Which side of the bubble the menu opens towards. */
  align: "start" | "end"
}

export default function MessageActionsMenu({ onReport, align }: MessageActionsMenuProps) {
  const t = useTranslations("chatRoom")
  const [open, setOpen] = useState(false)
  const wrapperRef = useRef<HTMLDivElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const menuId = useId()

  useMenuKeyboard({ open, containerRef: menuRef, onClose: () => setOpen(false) })

  useEffect(() => {
    if (!open) return
    function handlePointer(e: PointerEvent) {
      if (!wrapperRef.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener("pointerdown", handlePointer)
    return () => document.removeEventListener("pointerdown", handlePointer)
  }, [open])

  return (
    <div ref={wrapperRef} className="relative">
      <button
        type="button"
        aria-label={t("messageActions")}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        onClick={() => setOpen((o) => !o)}
        // The glyph is small to fit the meta row; the padding plus negative
        // margin keeps the hit area at touch size without moving the row.
        className="-m-2 flex cursor-pointer items-center justify-center rounded-full p-2 text-gray-600 transition-colors hover:text-gray-300 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-hover"
      >
        <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
          <circle cx="5" cy="12" r="1.75" />
          <circle cx="12" cy="12" r="1.75" />
          <circle cx="19" cy="12" r="1.75" />
        </svg>
      </button>

      {open && (
        <div
          ref={menuRef}
          id={menuId}
          role="menu"
          aria-label={t("messageActions")}
          className={`absolute bottom-full z-20 mb-2 min-w-[11rem] overflow-hidden rounded-xl bg-gray-800 shadow-2xl ring-1 ring-gray-700 ${align === "end" ? "right-0" : "left-0"}`}
        >
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setOpen(false)
              onReport()
            }}
            className="flex w-full cursor-pointer items-center gap-2 px-4 py-3 text-left text-sm text-red-400 hover:bg-gray-700 focus:bg-gray-700 focus:outline-none"
          >
            <svg className="h-4 w-4 shrink-0" fill="none" stroke="currentColor" strokeWidth="1.75" viewBox="0 0 24 24" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M3 3v18M3 4h13l-2 4 2 4H3" />
            </svg>
            {t("reportMessage")}
          </button>
        </div>
      )}
    </div>
  )
}
