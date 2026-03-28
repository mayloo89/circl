interface PresenceDotProps {
  online: boolean
  /**
   * sm — h-1.5 w-1.5, used inline next to status text (chat header).
   * md — h-2.5 w-2.5, used as an absolute badge on an avatar (contacts list).
   */
  size?: "sm" | "md"
  className?: string
}

export default function PresenceDot({ online, size = "md", className = "" }: PresenceDotProps) {
  const sizeClass  = size === "sm" ? "h-1.5 w-1.5" : "h-2.5 w-2.5"
  const colorClass = online ? "bg-green-400" : "bg-gray-600"
  return <span className={`${sizeClass} rounded-full ${colorClass} ${className}`} />
}
