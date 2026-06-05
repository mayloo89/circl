"use client"

import { useEffect, useRef } from "react"

const SITE_KEY = process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY
const SCRIPT_SRC = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit"

interface TurnstileApi {
  render: (el: HTMLElement, opts: Record<string, unknown>) => string
  remove: (id: string) => void
}

declare global {
  interface Window {
    turnstile?: TurnstileApi
  }
}

/** Whether the anti-bot challenge is configured (a site key is present). */
export const captchaEnabled = Boolean(SITE_KEY)

/**
 * Cloudflare Turnstile widget. Calls `onVerify` with the token when solved and
 * with "" when it expires or errors. Renders nothing when no site key is set
 * (local/dev), so the surrounding flow is unaffected — the backend likewise
 * skips verification when no secret is configured.
 *
 * Pass a stable `onVerify` (e.g. a `useState` setter); the effect depends on it.
 */
export default function Turnstile({ onVerify, resetTrigger }: { onVerify: (token: string) => void; resetTrigger?: number }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const widgetIdRef = useRef<string | null>(null)

  useEffect(() => {
    if (!SITE_KEY) return
    let cancelled = false

    function render() {
      if (cancelled || widgetIdRef.current || !containerRef.current || !window.turnstile) return
      widgetIdRef.current = window.turnstile.render(containerRef.current, {
        sitekey: SITE_KEY,
        theme: "dark",
        callback: (token: string) => onVerify(token),
        "error-callback": () => onVerify(""),
        "expired-callback": () => onVerify(""),
      })
    }

    if (window.turnstile) {
      render()
    } else {
      let script = document.querySelector<HTMLScriptElement>("script[data-turnstile]")
      if (!script) {
        script = document.createElement("script")
        script.src = SCRIPT_SRC
        script.async = true
        script.defer = true
        script.setAttribute("data-turnstile", "")
        document.head.appendChild(script)
      }
      script.addEventListener("load", render)
    }

    return () => {
      cancelled = true
      if (widgetIdRef.current && window.turnstile) {
        window.turnstile.remove(widgetIdRef.current)
      }
      widgetIdRef.current = null
    }
  }, [onVerify])

  // If `resetTrigger` changes, remove and re-render the widget to provide a
  // fresh challenge (used when the server rejects the nickname and we want
  // the user to solve a new captcha instance).
  useEffect(() => {
    if (!SITE_KEY) return
    if (resetTrigger === undefined) return
    if (!containerRef.current) return
    if (!window.turnstile) return

    if (widgetIdRef.current && window.turnstile) {
      try { window.turnstile.remove(widgetIdRef.current) } catch {}
      widgetIdRef.current = null
    }

    // Render a fresh widget instance and wire callbacks again.
    widgetIdRef.current = window.turnstile.render(containerRef.current, {
      sitekey: SITE_KEY,
      theme: "dark",
      callback: (token: string) => onVerify(token),
      "error-callback": () => onVerify(""),
      "expired-callback": () => onVerify(""),
    })
  }, [resetTrigger, onVerify])

  if (!SITE_KEY) return null
  return <div ref={containerRef} className="flex justify-center" />
}
