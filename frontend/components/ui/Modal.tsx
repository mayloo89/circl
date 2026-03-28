"use client"

import { useEffect } from "react"

interface ModalProps {
  open: boolean
  onClose: () => void
  children: React.ReactNode
}

/**
 * Base overlay modal. Closes on Escape key or backdrop click.
 * Render the close button and content as children — they become direct
 * children of the fixed-position backdrop, so absolute positioning works
 * as expected (e.g. close button at top-right of screen).
 */
export default function Modal({ open, onClose, children }: ModalProps) {
  useEffect(() => {
    if (!open) return
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose()
    }
    window.addEventListener("keydown", handleKey)
    return () => window.removeEventListener("keydown", handleKey)
  }, [open, onClose])

  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/90"
      onClick={onClose}
    >
      {children}
    </div>
  )
}
