"use client"

import { useState } from "react"
import { usePushContext } from "@/contexts/PushContext"

export default function PushPrompt() {
  const { permission, supported, enable } = usePushContext()
  const [dismissed, setDismissed] = useState(false)

  if (!supported || permission !== "default" || dismissed) return null

  return (
    <div className="flex items-center justify-between bg-indigo-600 px-6 py-2 text-sm text-white">
      <span>Enable notifications to stay updated on messages and contacts.</span>
      <div className="flex items-center gap-3">
        <button
          onClick={enable}
          className="rounded bg-white px-3 py-1 text-xs font-medium text-indigo-600 hover:bg-indigo-50"
        >
          Enable
        </button>
        <button
          onClick={() => setDismissed(true)}
          aria-label="Dismiss"
          className="text-indigo-200 hover:text-white"
        >
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>
    </div>
  )
}
