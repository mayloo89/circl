"use client"

import { useEffect, useRef, useCallback } from "react"

interface ModalProps {
  open: boolean
  onClose: () => void
  children: React.ReactNode
}

/**
 * Base overlay modal. Closes on Escape key or backdrop click.
 * Implements focus trap so Tab/Shift+Tab stays within the modal content.
 * Render the close button and content as children — they become direct
 * children of the fixed-position backdrop, so absolute positioning works
 * as expected (e.g. close button at top-right of screen).
 */
export default function Modal({ open, onClose, children }: ModalProps) {
  const backdropRef = useRef<HTMLDivElement>(null)

  const handleKey = useCallback((e: KeyboardEvent) => {
    if (e.key === "Escape") {
      onClose()
      return
    }
    if (e.key !== "Tab") return

    const container = backdropRef.current
    if (!container) return

    const focusable = container.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])'
    )
    if (focusable.length === 0) return

    const first = focusable[0]
    const last = focusable[focusable.length - 1]

    if (e.shiftKey) {
      if (document.activeElement === first) {
        e.preventDefault()
        last.focus()
      }
    } else {
      if (document.activeElement === last) {
        e.preventDefault()
        first.focus()
      }
    }
  }, [onClose])

  useEffect(() => {
    if (!open) return
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [open, handleKey])

  // Auto-focus first focusable element when opened
  useEffect(() => {
    if (!open) return
    const container = backdropRef.current
    if (!container) return
    // Delay to allow children to render
    const id = requestAnimationFrame(() => {
      const first = container.querySelector<HTMLElement>(
        'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])'
      )
      first?.focus()
    })
    return () => cancelAnimationFrame(id)
  }, [open])

  if (!open) return null

  return (
    <div
      ref={backdropRef}
      role="dialog"
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/90"
      onClick={onClose}
    >
      {children}
    </div>
  )
}
