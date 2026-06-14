"use client"

import { useEffect } from "react"
import { reportClientError } from "@/lib/reportError"

// global-error replaces the root layout when an error escapes it, so the
// i18n/provider tree isn't available here — this is the minimal last-resort
// fallback. Locale-aware errors are handled by app/[locale]/error.tsx.
export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  useEffect(() => {
    reportClientError({ error, kind: "boundary" })
  }, [error])

  return (
    <html lang="en" className="dark">
      <body
        style={{
          minHeight: "100vh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          margin: 0,
          fontFamily: "system-ui, sans-serif",
          background: "#0A1020",
          color: "#F4F9FF",
        }}
      >
        <div style={{ textAlign: "center", padding: "2rem", maxWidth: "32rem" }}>
          <h1 style={{ fontSize: "1.5rem", marginBottom: "0.75rem" }}>
            Something went wrong
          </h1>
          <p style={{ opacity: 0.8, marginBottom: "1.5rem" }}>
            An unexpected error occurred. Please try again.
          </p>
          <button
            type="button"
            onClick={reset}
            style={{
              padding: "0.625rem 1.25rem",
              borderRadius: "0.5rem",
              border: "none",
              background: "#4F7CFF",
              color: "#fff",
              fontSize: "1rem",
              cursor: "pointer",
            }}
          >
            Try again
          </button>
        </div>
      </body>
    </html>
  )
}
