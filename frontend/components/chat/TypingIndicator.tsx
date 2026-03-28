interface TypingIndicatorProps {
  typers: string[]
}

export default function TypingIndicator({ typers }: TypingIndicatorProps) {
  if (typers.length === 0) return null
  return (
    <div className="px-4 py-1 text-xs text-gray-400">
      {typers.join(", ")} {typers.length === 1 ? "is" : "are"} typing…
    </div>
  )
}
