"use client"

import { type InputHTMLAttributes, forwardRef } from "react"

export interface ToggleProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "type" | "size"> {
  /** Visible label for the toggle. Required for accessibility. */
  label: string
  /** Optional supporting copy displayed below the label. */
  description?: string
  /** Optional element rendered next to the label (e.g. a "Symmetric" badge). */
  badge?: React.ReactNode
}

// The native checkbox is rendered transparent and absolutely positioned over
// the visible track — not `sr-only` — so its bounding box matches the track.
// This matters because `<main>` is the page's scroll container (set in
// AppShell), and a focused element with a 1×1 box at the top-left of its
// label triggers the browser's scroll-into-view behaviour, which would
// otherwise jump the page on every toggle.
const Toggle = forwardRef<HTMLInputElement, ToggleProps>(function Toggle(
  { label, description, badge, checked, disabled, className = "", id, ...rest },
  ref,
) {
  const inputId = id ?? `toggle-${label.replace(/\s+/g, "-").toLowerCase()}`
  const trackBase =
    "relative inline-flex h-6 w-11 rounded-full ring-1 transition-colors"
  const trackOff = "bg-gray-700 ring-gray-600"
  const trackOn = "bg-brand-primary ring-brand-hover"
  const thumbBase =
    "absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white shadow-sm transition-transform"

  return (
    <label
      htmlFor={inputId}
      className={`flex min-h-[44px] cursor-pointer items-start justify-between gap-4 ${
        disabled ? "cursor-not-allowed opacity-60" : ""
      } ${className}`}
    >
      <span className="flex flex-col gap-0.5 pt-0.5">
        <span className="flex items-center gap-2 text-sm font-medium text-gray-200">
          {label}
          {badge}
        </span>
        {description && <span className="text-xs text-gray-500">{description}</span>}
      </span>

      <span className="relative inline-flex shrink-0 self-center">
        <input
          ref={ref}
          id={inputId}
          type="checkbox"
          role="switch"
          aria-checked={checked}
          checked={checked}
          disabled={disabled}
          className="peer absolute inset-0 z-10 m-0 cursor-pointer appearance-none rounded-full opacity-0 disabled:cursor-not-allowed"
          {...rest}
        />
        <span
          aria-hidden="true"
          className={`${trackBase} ${checked ? trackOn : trackOff} peer-focus-visible:outline peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-brand-hover`}
        >
          <span className={`${thumbBase} ${checked ? "translate-x-5" : "translate-x-0"}`} />
        </span>
      </span>
    </label>
  )
})

export default Toggle
