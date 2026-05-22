"use client"

import { useRef } from "react"
import { useTranslations } from "next-intl"

import { useFocusTrap } from "@/hooks/useFocusTrap"

interface BottomSheetProps {
  open: boolean
  onClose: () => void
  title?: string
  children: React.ReactNode
}

export default function BottomSheet({ open, onClose, title, children }: BottomSheetProps) {
  const tc = useTranslations("common")
  const sheetRef = useRef<HTMLDivElement>(null)

  useFocusTrap({ active: open, containerRef: sheetRef, onEscape: onClose })

  if (!open) return null

  return (
    <div className="lg:hidden">
      {/* Backdrop */}
      <div
        className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm"
        onClick={onClose}
        aria-hidden="true"
      />
      {/* Sheet */}
      <div
        ref={sheetRef}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className="fixed inset-x-0 bottom-0 z-[60] flex max-h-[85dvh] flex-col rounded-t-2xl bg-gray-900 ring-1 ring-gray-800 animate-slide-up"
      >
        <div className="flex flex-none items-center justify-between border-b border-gray-800 px-4 py-3">
          {title && <p className="text-sm font-semibold text-foreground">{title}</p>}
          <button
            onClick={onClose}
            className="ml-auto cursor-pointer rounded p-3 text-gray-400 transition-colors hover:text-white focus:outline-none focus:ring-2 focus:ring-brand-hover"
            aria-label={tc("close")}
          >
            <svg className="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
        <div className="flex-1 overflow-y-auto p-4">
          {children}
        </div>
      </div>
    </div>
  )
}
