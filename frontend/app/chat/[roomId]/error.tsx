"use client"

export default function Error({ error, reset }: { error: Error; reset: () => void }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-4 bg-gray-950 px-4">
      <p className="rounded-md bg-red-950 p-4 text-sm text-red-400 ring-1 ring-red-900">
        {error.message || "Something went wrong loading this conversation."}
      </p>
      <button
        onClick={reset}
        className="rounded bg-gray-700 px-4 py-2 text-sm text-gray-200 hover:bg-gray-600"
      >
        Try again
      </button>
    </div>
  )
}
