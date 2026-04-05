"use client"

import { usePushContext } from "@/contexts/PushContext"

export default function PushPrompt() {
  const { permission, supported, enable } = usePushContext()

  if (!supported || permission !== "default") return null

  return (
    <div className="flex items-center justify-between bg-indigo-600 px-6 py-2 text-sm text-white">
      <span>Enable notifications to stay updated on messages and contacts.</span>
      <div className="flex items-center gap-4">
        <button
          onClick={enable}
          className="rounded bg-white px-3 py-1 text-xs font-medium text-indigo-600 hover:bg-indigo-50"
        >
          Enable
        </button>
      </div>
    </div>
  )
}
