const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export type ClientErrorKind = "error" | "unhandledrejection" | "boundary"

interface ClientErrorReport {
  message: string
  stack?: string
  url?: string
  kind: ClientErrorKind
}

// reportClientError forwards a browser-side error to the backend ingest
// endpoint, which logs it (event=client_error) into Loki/Grafana. Best-effort
// and never throws: a failure to report must not cascade into more errors.
// `keepalive` lets the request survive a navigation/unload triggered by the
// error.
export function reportClientError(input: {
  error: unknown
  kind: ClientErrorKind
}): void {
  if (typeof window === "undefined") return

  const { error, kind } = input
  const message =
    error instanceof Error ? error.message : String(error ?? "unknown error")
  const stack = error instanceof Error ? error.stack : undefined

  const report: ClientErrorReport = {
    message,
    stack,
    url: window.location?.href,
    kind,
  }

  try {
    void fetch(`${API_URL}/client-errors`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(report),
      keepalive: true,
    }).catch(() => {})
  } catch {
    // Swallow — reporting is best-effort.
  }
}
