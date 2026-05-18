"use client"

import { useEffect, useState } from "react"

interface Props {
  src: string
  token: string
  alt: string
  className?: string
}

type Status =
  | { kind: "loading" }
  | { kind: "ok"; url: string }
  | { kind: "error" }

/**
 * AuthedImage fetches `src` with the Bearer token, turns the response into a
 * blob URL, and renders it. Used in admin surfaces where the underlying
 * endpoint requires the Authorization header — a plain `<img src>` would
 * send the cookie but not the Bearer header, so the request 401s.
 *
 * Callers should pass a stable `key` (e.g. the upload id) so React remounts
 * the component when the underlying resource changes, instead of relying on
 * effect-driven state resets.
 */
export default function AuthedImage({ src, token, alt, className }: Props) {
  const [status, setStatus] = useState<Status>({ kind: "loading" })

  useEffect(() => {
    let cancelled = false
    let createdUrl: string | null = null
    fetch(src, { headers: { Authorization: `Bearer ${token}` } })
      .then(async (res) => {
        if (cancelled) return
        if (!res.ok) {
          setStatus({ kind: "error" })
          return
        }
        const blob = await res.blob()
        if (cancelled) return
        createdUrl = URL.createObjectURL(blob)
        setStatus({ kind: "ok", url: createdUrl })
      })
      .catch(() => {
        if (!cancelled) setStatus({ kind: "error" })
      })
    return () => {
      cancelled = true
      if (createdUrl) URL.revokeObjectURL(createdUrl)
    }
  }, [src, token])

  if (status.kind === "error") {
    return (
      <div
        className={`flex items-center justify-center bg-gray-800 text-[10px] text-gray-500 ${className ?? ""}`}
        aria-label={alt}
      >
        N/A
      </div>
    )
  }
  if (status.kind === "loading") {
    return <div className={`animate-pulse bg-gray-800 ${className ?? ""}`} aria-label={alt} />
  }
  // eslint-disable-next-line @next/next/no-img-element -- blob URL, next/image can't optimize it
  return <img src={status.url} alt={alt} className={className} />
}
