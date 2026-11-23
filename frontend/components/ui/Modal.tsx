"use client"

import { useRef } from "react"

import { useFocusTrap } from "@/hooks/useFocusTrap"

interface ModalProps {
  open: boolean
  onClose: () => void
  children: React.ReactNode
  /** Id of the element (usually the heading) that names the dialog. */
  labelledBy?: string
}

/**
 * Base overlay modal. Closes on Escape key or backdrop click.
 *
 * Focus management (capture trigger, auto-focus first focusable, trap
 * Tab/Shift-Tab, restore focus on close, lock body scroll) is delegated to
 * `useFocusTrap` so behaviour stays consistent across every dialog surface.
 */
export default function Modal({ open, onClose, children, labelledBy }: ModalProps) {
  const backdropRef = useRef<HTMLDivElement>(null)

  useFocusTrap({ active: open, containerRef: backdropRef, onEscape: onClose })

  if (!open) return null

  return (
    <div
      ref={backdropRef}
      role="dialog"
      aria-modal="true"
      aria-labelledby={labelledBy}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/90"
      onClick={onClose}
    >
      {children}
    </div>
  )
}
