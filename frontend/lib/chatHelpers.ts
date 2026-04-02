import type { AnyMessage } from "@/types/chat"

const GROUP_BREAK_MS = 5 * 60_000

export function sameCalendarDay(a: string, b: string): boolean {
  const da = new Date(a), db = new Date(b)
  return (
    da.getFullYear() === db.getFullYear() &&
    da.getMonth() === db.getMonth() &&
    da.getDate() === db.getDate()
  )
}

export function formatDaySeparator(dateStr: string, nowMs: number): string {
  const now = new Date(nowMs)
  const yesterday = new Date(nowMs)
  yesterday.setDate(now.getDate() - 1)
  if (sameCalendarDay(dateStr, now.toISOString())) return "Today"
  if (sameCalendarDay(dateStr, yesterday.toISOString())) return "Yesterday"
  return new Date(dateStr).toLocaleDateString("en", { weekday: "long", month: "long", day: "numeric" })
}

export function isFirstInGroup(msgs: AnyMessage[], i: number): boolean {
  if (i === 0) return true
  const prev = msgs[i - 1]
  const cur = msgs[i]
  if (prev.sender_id !== cur.sender_id) return true
  if (!sameCalendarDay(prev.created_at, cur.created_at)) return true
  if (new Date(cur.created_at).getTime() - new Date(prev.created_at).getTime() > GROUP_BREAK_MS) return true
  if (prev.tombstone) return true
  return false
}

export function isLastInGroup(msgs: AnyMessage[], i: number): boolean {
  if (i === msgs.length - 1) return true
  const cur = msgs[i]
  const next = msgs[i + 1]
  if (cur.sender_id !== next.sender_id) return true
  if (!sameCalendarDay(cur.created_at, next.created_at)) return true
  if (new Date(next.created_at).getTime() - new Date(cur.created_at).getTime() > GROUP_BREAK_MS) return true
  if (cur.tombstone) return true
  return false
}

export function formatExpiry(expiresAt: string, nowMs: number): string {
  const diff = new Date(expiresAt).getTime() - nowMs
  if (diff <= 0) return "expired"
  const h = Math.floor(diff / 3_600_000)
  const m = Math.floor((diff % 3_600_000) / 60_000)
  if (h >= 24) return `${Math.floor(h / 24)}d left`
  if (h >= 1) return `${h}h left`
  if (m >= 1) return `${m}m left`
  return "< 1m"
}

export function expiryColorClass(expiresAt: string, nowMs: number): string {
  const diff = new Date(expiresAt).getTime() - nowMs
  const mins = diff / 60_000
  if (mins < 10) return "text-red-400"
  if (mins < 60) return "text-amber-400"
  return "text-gray-500"
}
