"use client"

import { type InputHTMLAttributes, type ReactNode, forwardRef, useId } from "react"

export interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "type" | "size"> {
  /** Visible label. ReactNode so embedded links / formatting are supported. */
  label: ReactNode
  /** Error message. When present, the input is marked aria-invalid and the
   * message is wired via aria-describedby. */
  error?: string
}

const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(function Checkbox(
  { label, error, checked, disabled, className = "", id, ...rest },
  ref,
) {
  const reactId = useId()
  const inputId = id ?? reactId
  const errorId = `${inputId}-error`

  return (
    <div className={className}>
      <label
        htmlFor={inputId}
        className={`flex min-h-[44px] items-start gap-3 ${
          disabled ? "cursor-not-allowed opacity-60" : "cursor-pointer"
        }`}
      >
        <span className="relative inline-flex h-6 w-6 shrink-0 self-center">
          <input
            ref={ref}
            id={inputId}
            type="checkbox"
            checked={checked}
            disabled={disabled}
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? errorId : undefined}
            className="peer absolute inset-0 z-10 m-0 h-full w-full cursor-pointer appearance-none rounded disabled:cursor-not-allowed"
            {...rest}
          />
          <span
            aria-hidden="true"
            className={`flex h-5 w-5 self-center items-center justify-center rounded border-2 transition-colors ${
              checked
                ? "border-brand-primary bg-brand-primary"
                : error
                  ? "border-red-500 bg-transparent"
                  : "border-gray-600 bg-transparent"
            } peer-focus-visible:outline peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-brand-hover`}
          >
            {checked && (
              <svg className="h-3 w-3 text-white" fill="none" stroke="currentColor" strokeWidth="3" viewBox="0 0 24 24" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M4.5 12.75l6 6 9-13.5" />
              </svg>
            )}
          </span>
        </span>
        <span className="select-none text-sm leading-snug text-gray-300">
          {label}
        </span>
      </label>
      {error && (
        <p id={errorId} className="ml-9 mt-1 text-xs text-red-400" role="alert">
          {error}
        </p>
      )}
    </div>
  )
})

export default Checkbox
