import React from "react"

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string
  /** Visually hides the label while keeping it accessible. */
  labelHidden?: boolean
  error?: string
  helper?: string
  /** Renders an orange border to indicate unsaved changes. */
  dirty?: boolean
}

export default function Input({
  label,
  labelHidden = false,
  error,
  helper,
  dirty = false,
  id,
  className = "",
  ...props
}: InputProps) {
  const borderClass = dirty
    ? "border-orange-500 focus:border-orange-400 focus:ring-orange-400"
    : "border-gray-700 focus:border-brand-hover focus:ring-brand-hover"

  return (
    <div>
      {label && (
        <label
          htmlFor={id}
          className={`block text-sm font-medium text-gray-300 ${labelHidden ? "sr-only" : ""}`}
        >
          {label}
        </label>
      )}
      <input
        id={id}
        className={`${label && !labelHidden ? "mt-1 " : ""}block w-full rounded-md border bg-gray-800 px-3 py-2 text-white placeholder-gray-500 shadow-sm focus:outline-none focus:ring-1 ${borderClass} ${className}`}
        {...props}
      />
      {error  && <p className="mt-1 text-xs text-red-400">{error}</p>}
      {helper && !error && <p className="mt-1 text-xs text-gray-500">{helper}</p>}
    </div>
  )
}
