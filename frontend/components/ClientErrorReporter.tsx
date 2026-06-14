"use client"

import { useEffect } from "react"
import { reportClientError } from "@/lib/reportError"

// Captures errors that escape React's boundaries — uncaught exceptions and
// unhandled promise rejections — and forwards them to the backend ingest
// endpoint. Renders nothing.
export default function ClientErrorReporter() {
  useEffect(() => {
    const onError = (event: ErrorEvent) => {
      reportClientError({ error: event.error ?? event.message, kind: "error" })
    }
    const onRejection = (event: PromiseRejectionEvent) => {
      reportClientError({ error: event.reason, kind: "unhandledrejection" })
    }
    window.addEventListener("error", onError)
    window.addEventListener("unhandledrejection", onRejection)
    return () => {
      window.removeEventListener("error", onError)
      window.removeEventListener("unhandledrejection", onRejection)
    }
  }, [])

  return null
}
