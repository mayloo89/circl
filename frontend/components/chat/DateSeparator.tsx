interface DateSeparatorProps {
  label: string
}

export default function DateSeparator({ label }: DateSeparatorProps) {
  return (
    <div className="my-4 flex items-center gap-3">
      <div className="h-px flex-1 bg-gray-800" />
      <span className="text-[11px] font-medium text-gray-500">{label}</span>
      <div className="h-px flex-1 bg-gray-800" />
    </div>
  )
}
