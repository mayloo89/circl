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

export function formatDaySeparator(
  dateStr: string,
  nowMs: number,
  locale: string,
  labels: { today: string; yesterday: string },
): string {
  const now = new Date(nowMs)
  const yesterday = new Date(nowMs)
  yesterday.setDate(now.getDate() - 1)
  if (sameCalendarDay(dateStr, now.toISOString())) return labels.today
  if (sameCalendarDay(dateStr, yesterday.toISOString())) return labels.yesterday
  return new Date(dateStr).toLocaleDateString(locale, { weekday: "long", month: "long", day: "numeric" })
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

export type ExpiryCountdown =
  | { unit: "expired" | "underMinute" }
  | { unit: "days" | "hours" | "minutes"; value: number }

export function expiryCountdown(expiresAt: string, nowMs: number): ExpiryCountdown {
  const diff = new Date(expiresAt).getTime() - nowMs
  if (diff <= 0) return { unit: "expired" }
  const h = Math.floor(diff / 3_600_000)
  const m = Math.floor((diff % 3_600_000) / 60_000)
  if (h >= 24) return { unit: "days", value: Math.floor(h / 24) }
  if (h >= 1) return { unit: "hours", value: h }
  if (m >= 1) return { unit: "minutes", value: m }
  return { unit: "underMinute" }
}

export function expiryColorClass(expiresAt: string, nowMs: number): string {
  const diff = new Date(expiresAt).getTime() - nowMs
  const mins = diff / 60_000
  if (mins < 10) return "text-red-400"
  if (mins < 60) return "text-amber-400"
  return "text-gray-500"
}
