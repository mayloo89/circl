"use client"

import Image from "next/image"

const sizes = {
  xs: { box: "h-6 w-6",   text: "text-xs",           px: 24, ring: "ring-1 ring-gray-700", fallbackRing: "ring-1 ring-gray-600" },
  sm: { box: "h-7 w-7",   text: "text-xs",           px: 28, ring: "ring-1 ring-gray-700", fallbackRing: "ring-1 ring-gray-600" },
  md: { box: "h-8 w-8",   text: "text-sm",           px: 32, ring: "ring-1 ring-gray-700", fallbackRing: "ring-1 ring-gray-600" },
  lg: { box: "h-10 w-10", text: "text-sm font-medium", px: 40, ring: "ring-1 ring-gray-700", fallbackRing: "ring-1 ring-gray-600" },
  xl: { box: "h-24 w-24", text: "text-3xl",          px: 96, ring: "ring-2 ring-gray-700", fallbackRing: "ring-2 ring-gray-600" },
}

interface AvatarProps {
  src?: string
  name: string
  size?: keyof typeof sizes
  /** Override the fallback circle color. "gray" is the default for users; "indigo" for group rooms. */
  color?: "gray" | "indigo"
  className?: string
}

export default function Avatar({ src, name, size = "md", color = "gray", className = "" }: AvatarProps) {
  const { box, text, px, ring, fallbackRing } = sizes[size]
  const initial = (name || "?")[0].toUpperCase()

  if (src) {
    return (
      <Image
        src={src}
        alt={name ?? "User avatar"}
        width={px}
        height={px}
        className={`${box} flex-none rounded-full object-cover ${ring} ${className}`}
      />
    )
  }

  const bgClass      = color === "indigo" ? "bg-brand-strong"   : "bg-gray-700"
  const textClass    = color === "indigo" ? "text-white"      : "text-gray-300"
  const ringOverride = color === "indigo" ? "ring-1 ring-brand-primary" : fallbackRing

  return (
    <span
      className={`${box} flex flex-none items-center justify-center rounded-full ${bgClass} ${text} ${textClass} ${ringOverride} ${className}`}
    >
      {initial}
    </span>
  )
}
