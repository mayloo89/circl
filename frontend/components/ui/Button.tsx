import React from "react"

type Variant = "primary" | "secondary" | "danger" | "success" | "ghost" | "warning"
type Size    = "sm" | "md" | "lg"

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  /** Renders with rounded-full instead of rounded. */
  pill?: boolean
  /** Shows a spinner and disables the button. */
  loading?: boolean
}

const variantClasses: Record<Variant, string> = {
  primary:   "bg-indigo-600 hover:bg-indigo-500 text-white focus:ring-indigo-500",
  secondary: "bg-gray-700 hover:bg-gray-600 text-gray-200 focus:ring-gray-500",
  danger:    "bg-red-900 hover:bg-red-800 text-red-300 focus:ring-red-500",
  success:   "bg-green-700 hover:bg-green-600 text-white focus:ring-green-500",
  ghost:     "text-gray-400 hover:text-gray-200 focus:ring-gray-500",
  warning:   "bg-orange-600 hover:bg-orange-500 text-white focus:ring-orange-500",
}

const sizeClasses: Record<Size, string> = {
  sm: "px-3 py-2 text-xs",
  md: "px-4 py-2 text-sm",
  lg: "px-5 py-2.5 text-sm",
}

function Spinner() {
  return (
    <svg className="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24" aria-hidden="true">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
    </svg>
  )
}

export default function Button({
  variant = "primary",
  size = "md",
  pill = false,
  loading = false,
  disabled,
  className = "",
  children,
  ...props
}: ButtonProps) {
  // ghost buttons carry no background/padding — size prop is ignored.
  const sizeClass  = variant === "ghost" ? "text-sm" : sizeClasses[size]
  const roundClass = pill ? "rounded-full" : "rounded"
  const base       = "inline-flex items-center justify-center gap-2 font-medium transition-colors disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-gray-900"

  return (
    <button
      disabled={disabled || loading}
      className={`${base} ${variantClasses[variant]} ${sizeClass} ${roundClass} ${className}`}
      {...props}
    >
      {loading && <Spinner />}
      {children}
    </button>
  )
}
