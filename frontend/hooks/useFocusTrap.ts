"use client"

import { type RefObject, useEffect } from "react"

const FOCUSABLE_SELECTORS = [
  "a[href]",
  "button:not([disabled])",
  "textarea:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  '[tabindex]:not([tabindex="-1"])',
].join(",")

export interface UseFocusTrapOptions {
  /**
   * When true, the trap is active. Toggling false restores focus to the
   * element that had focus when the trap most recently activated.
   */
  active: boolean
  /**
   * Element that holds the focusable content. Tab/Shift-Tab will be trapped
   * inside it; the first focusable child receives focus when active flips on.
   */
  containerRef: RefObject<HTMLElement | null>
  /**
   * Called when the user presses Escape. If undefined, Escape is ignored.
   */
  onEscape?: () => void
  /**
   * Lock body scroll while the trap is active. Defaults to true so background
   * content can't scroll behind the dialog. Set false for inline dialogs.
   */
  lockBodyScroll?: boolean
}

/**
 * Centralised focus management for dialog-like overlays (modal, sheet,
 * full-screen panel). Captures the currently-focused element on activation,
 * moves focus into the container, traps Tab/Shift-Tab inside it, optionally
 * handles Escape, locks body scroll, and restores focus to the original
 * trigger when the trap deactivates.
 *
 * Mount the consuming component conditionally (e.g. {open && <Modal/>}) or
 * gate via `active` — both work.
 */
export function useFocusTrap({
  active,
  containerRef,
  onEscape,
  lockBodyScroll = true,
}: UseFocusTrapOptions) {
  // Capture trigger + auto-focus first focusable on activation; restore on
  // deactivation. Bundled in one effect so the cleanup pairs with the open
  // event that captured the trigger.
  useEffect(() => {
    if (!active) return
    const previouslyFocused = document.activeElement as HTMLElement | null
    const container = containerRef.current
    let frame = 0
    if (container) {
      // Wait one frame so children have rendered before searching.
      frame = requestAnimationFrame(() => {
        const first = container.querySelector<HTMLElement>(FOCUSABLE_SELECTORS)
        first?.focus()
      })
    }
    return () => {
      if (frame) cancelAnimationFrame(frame)
      if (previouslyFocused && typeof previouslyFocused.focus === "function") {
        previouslyFocused.focus()
      }
    }
  }, [active, containerRef])

  // Tab trap + Escape.
  useEffect(() => {
    if (!active) return
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape" && onEscape) {
        onEscape()
        return
      }
      if (e.key !== "Tab") return
      const container = containerRef.current
      if (!container) return
      const focusable = container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTORS)
      if (focusable.length === 0) {
        // Nothing to focus — keep focus at the container itself so Tab
        // doesn't leak to background content.
        e.preventDefault()
        return
      }
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      const inside = container.contains(document.activeElement)
      if (e.shiftKey) {
        if (!inside || document.activeElement === first) {
          e.preventDefault()
          last.focus()
        }
      } else {
        if (!inside || document.activeElement === last) {
          e.preventDefault()
          first.focus()
        }
      }
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [active, containerRef, onEscape])

  // Body scroll lock — composed-friendly: each active hook pushes onto the
  // existing inline style and restores it on unmount, so nested dialogs work.
  useEffect(() => {
    if (!active || !lockBodyScroll) return
    const previous = document.body.style.overflow
    document.body.style.overflow = "hidden"
    return () => { document.body.style.overflow = previous }
  }, [active, lockBodyScroll])
}
