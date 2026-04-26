interface BadgeProps {
  count: number
  /** Upper bound for display. Counts above this show as "{max}+". */
  max?: number
  /**
   * dot    — compact h-4 w-4 pill, used for nav link overlays (max defaults to 9).
   * count  — h-5 min-w-5 pill, used for unread message counts (default).
   * pill   — inline text pill with horizontal padding, used beside section headings.
   */
  variant?: "dot" | "count" | "pill"
  className?: string
}

export default function Badge({ count, max, variant = "count", className = "" }: BadgeProps) {
  const effective = max !== undefined && count > max ? `${max}+` : String(count)

  const base = "rounded-full bg-brand-primary font-semibold text-white"

  if (variant === "dot") {
    return (
      <span className={`${base} flex h-4 w-4 items-center justify-center text-[10px] ${className}`}>
        {effective}
      </span>
    )
  }

  if (variant === "pill") {
    return (
      <span className={`${base} px-2 py-0.5 text-xs ${className}`}>
        {effective}
      </span>
    )
  }

  // count (default)
  return (
    <span className={`${base} flex h-5 min-w-5 items-center justify-center px-1.5 text-xs ${className}`}>
      {effective}
    </span>
  )
}
