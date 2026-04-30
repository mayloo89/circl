"use client"

import { useState } from "react"
import type { InputProps } from "./Input"

function EyeIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
      <path strokeLinecap="round" strokeLinejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" />
      <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
    </svg>
  )
}

function EyeSlashIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24" aria-hidden="true">
      <path strokeLinecap="round" strokeLinejoin="round" d="M3.98 8.223A10.477 10.477 0 0 0 1.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.451 10.451 0 0 1 12 4.5c4.756 0 8.773 3.162 10.065 7.498a10.522 10.522 0 0 1-4.293 5.774M6.228 6.228 3 3m3.228 3.228 3.65 3.65m7.894 7.894L21 21m-3.228-3.228-3.65-3.65m0 0a3 3 0 1 0-4.243-4.243m4.242 4.242L9.88 9.88" />
    </svg>
  )
}

export type PasswordFieldProps = Omit<InputProps, "type">

export default function PasswordField({
  label,
  labelHidden = false,
  error,
  helper,
  dirty = false,
  id,
  className = "",
  ...props
}: PasswordFieldProps) {
  const [visible, setVisible] = useState(false)

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
      <div className={`relative${label && !labelHidden ? " mt-1" : ""}`}>
        <input
          id={id}
          type={visible ? "text" : "password"}
          className={`block w-full rounded-md border bg-gray-800 px-3 py-2 pr-10 text-white placeholder-gray-500 shadow-sm focus:outline-none focus:ring-1 ${borderClass} ${className}`}
          {...props}
        />
        <button
          type="button"
          onClick={() => setVisible((v) => !v)}
          tabIndex={-1}
          aria-label={visible ? "Hide password" : "Show password"}
          className="absolute inset-y-0 right-0 flex items-center px-3 text-gray-400 hover:text-gray-200 focus:outline-none"
        >
          {visible
            ? <EyeSlashIcon className="h-4 w-4" />
            : <EyeIcon className="h-4 w-4" />
          }
        </button>
      </div>
      {error  && <p className="mt-1 text-xs text-red-400">{error}</p>}
      {helper && !error && <p className="mt-1 text-xs text-gray-500">{helper}</p>}
    </div>
  )
}
