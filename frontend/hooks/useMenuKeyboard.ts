"use client"

import { type RefObject, useEffect } from "react"

export interface UseMenuKeyboardOptions {
  /** When true, the dropdown is open. Toggling to false restores focus to the trigger. */
  open: boolean
  /**
   * Container holding the menuitems. Items are discovered via the
   * `[role="menuitem"]` selector, so make sure menu options have that role.
   */
  containerRef: RefObject<HTMLElement | null>
  /** Called on Escape and on Tab (which closes the menu and lets focus continue past). */
  onClose: () => void
}

/**
 * Implements WAI-ARIA menu keyboard semantics for a click-to-open dropdown:
 *   - First menuitem receives focus when the menu opens
 *   - Escape and Tab/Shift-Tab close the menu (and Tab lets focus continue past it)
 *   - ArrowDown / ArrowUp wrap through menuitems
 *   - Home / End jump to first / last
 *   - Focus is restored to the original trigger element when the menu closes
 *
 * Tab is intentionally NOT trapped (unlike `useFocusTrap` for dialogs) — menus
 * are part of the page's tab order and should yield focus to surrounding
 * controls when the user tabs out.
 */
export function useMenuKeyboard({ open, containerRef, onClose }: UseMenuKeyboardOptions) {
  // Capture trigger + auto-focus first menuitem on open; restore on close.
  useEffect(() => {
    if (!open) return
    const previouslyFocused = document.activeElement as HTMLElement | null
    const container = containerRef.current
    let frame = 0
    if (container) {
      frame = requestAnimationFrame(() => {
        const items = container.querySelectorAll<HTMLElement>('[role="menuitem"]:not([disabled])')
        items[0]?.focus()
      })
    }
    return () => {
      if (frame) cancelAnimationFrame(frame)
      if (previouslyFocused && typeof previouslyFocused.focus === "function") {
        previouslyFocused.focus()
      }
    }
  }, [open, containerRef])

  // Keyboard navigation while open.
  useEffect(() => {
    if (!open) return
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault()
        onClose()
        return
      }
      if (e.key === "Tab") {
        // Don't trap Tab — close the menu and let focus continue naturally.
        onClose()
        return
      }
      const container = containerRef.current
      if (!container) return
      const items = Array.from(
        container.querySelectorAll<HTMLElement>('[role="menuitem"]:not([disabled])')
      )
      if (items.length === 0) return
      const currentIdx = items.findIndex((el) => el === document.activeElement)
      if (e.key === "ArrowDown") {
        e.preventDefault()
        const next = currentIdx === -1 ? 0 : (currentIdx + 1) % items.length
        items[next].focus()
      } else if (e.key === "ArrowUp") {
        e.preventDefault()
        const prev = currentIdx === -1 ? items.length - 1 : (currentIdx - 1 + items.length) % items.length
        items[prev].focus()
      } else if (e.key === "Home") {
        e.preventDefault()
        items[0].focus()
      } else if (e.key === "End") {
        e.preventDefault()
        items[items.length - 1].focus()
      }
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [open, containerRef, onClose])
}
