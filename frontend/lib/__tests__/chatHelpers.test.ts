import { describe, it, expect } from "vitest"
import {
  sameCalendarDay,
  formatDaySeparator,
  isFirstInGroup,
  isLastInGroup,
  formatExpiry,
  expiryColorClass,
} from "@/lib/chatHelpers"
import type { AnyMessage } from "@/types/chat"

// ─── Helpers ────────────────────────────────────────────────────────────────

function msg(
  id: string,
  sender_id: string,
  created_at: string,
  extra: Partial<AnyMessage> = {},
): AnyMessage {
  return {
    id,
    room_id: "r1",
    sender_id,
    sender_name: "User",
    sender_avatar_url: "",
    type: "text",
    content: "hi",
    view_once: false,
    created_at,
    ...extra,
  }
}

// ─── sameCalendarDay ────────────────────────────────────────────────────────

describe("sameCalendarDay", () => {
  it("returns true for two times on the same day", () => {
    expect(sameCalendarDay("2024-06-15T10:00:00Z", "2024-06-15T22:00:00Z")).toBe(true)
  })

  it("returns false for consecutive days", () => {
    expect(sameCalendarDay("2024-06-15T12:00:00Z", "2024-06-16T12:00:00Z")).toBe(false)
  })

  it("returns false for different months", () => {
    expect(sameCalendarDay("2024-01-15T12:00:00Z", "2024-02-15T12:00:00Z")).toBe(false)
  })

  it("returns false for different years", () => {
    expect(sameCalendarDay("2024-06-15T12:00:00Z", "2025-06-15T12:00:00Z")).toBe(false)
  })
})

// ─── formatDaySeparator ─────────────────────────────────────────────────────

describe("formatDaySeparator", () => {
  it("returns 'Today' for the current date", () => {
    const now = new Date()
    const todayNoon = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 12, 0, 0)
    expect(formatDaySeparator(todayNoon.toISOString(), now.getTime())).toBe("Today")
  })

  it("returns 'Yesterday' for the previous date", () => {
    const now = new Date()
    const yesterday = new Date(now)
    yesterday.setDate(yesterday.getDate() - 1)
    const d = new Date(yesterday.getFullYear(), yesterday.getMonth(), yesterday.getDate(), 12, 0, 0)
    expect(formatDaySeparator(d.toISOString(), now.getTime())).toBe("Yesterday")
  })

  it("returns a formatted string for dates older than yesterday", () => {
    const old = "2020-06-15T12:00:00Z"
    const result = formatDaySeparator(old, Date.now())
    expect(result).not.toBe("Today")
    expect(result).not.toBe("Yesterday")
    expect(result.length).toBeGreaterThan(0)
  })
})

// ─── isFirstInGroup ─────────────────────────────────────────────────────────

describe("isFirstInGroup", () => {
  it("is true for the first message in the list", () => {
    const msgs = [msg("1", "alice", "2024-06-15T12:00:00Z")]
    expect(isFirstInGroup(msgs, 0)).toBe(true)
  })

  it("is true when sender changes", () => {
    const msgs = [
      msg("1", "alice", "2024-06-15T12:00:00Z"),
      msg("2", "bob", "2024-06-15T12:01:00Z"),
    ]
    expect(isFirstInGroup(msgs, 1)).toBe(true)
  })

  it("is false when same sender within 5-minute window", () => {
    const msgs = [
      msg("1", "alice", "2024-06-15T12:00:00Z"),
      msg("2", "alice", "2024-06-15T12:03:00Z"),
    ]
    expect(isFirstInGroup(msgs, 1)).toBe(false)
  })

  it("is true when same sender but gap exceeds 5 minutes", () => {
    const msgs = [
      msg("1", "alice", "2024-06-15T12:00:00Z"),
      msg("2", "alice", "2024-06-15T12:06:00Z"),
    ]
    expect(isFirstInGroup(msgs, 1)).toBe(true)
  })

  it("is true when messages span a day boundary", () => {
    // Use local dates to avoid timezone-dependent UTC midnight issues
    const d1 = new Date(2024, 5, 15, 23, 59, 0) // local June 15 11:59pm
    const d2 = new Date(2024, 5, 16, 0, 1, 0)   // local June 16 12:01am
    const msgs = [msg("1", "alice", d1.toISOString()), msg("2", "alice", d2.toISOString())]
    expect(isFirstInGroup(msgs, 1)).toBe(true)
  })

  it("is true when previous message is a tombstone", () => {
    const msgs = [
      msg("1", "alice", "2024-06-15T12:00:00Z", { tombstone: true }),
      msg("2", "alice", "2024-06-15T12:01:00Z"),
    ]
    expect(isFirstInGroup(msgs, 1)).toBe(true)
  })
})

// ─── isLastInGroup ──────────────────────────────────────────────────────────

describe("isLastInGroup", () => {
  it("is true for the last message in the list", () => {
    const msgs = [msg("1", "alice", "2024-06-15T12:00:00Z")]
    expect(isLastInGroup(msgs, 0)).toBe(true)
  })

  it("is true when next message has a different sender", () => {
    const msgs = [
      msg("1", "alice", "2024-06-15T12:00:00Z"),
      msg("2", "bob", "2024-06-15T12:01:00Z"),
    ]
    expect(isLastInGroup(msgs, 0)).toBe(true)
  })

  it("is false when same sender within 5-minute window", () => {
    const msgs = [
      msg("1", "alice", "2024-06-15T12:00:00Z"),
      msg("2", "alice", "2024-06-15T12:03:00Z"),
    ]
    expect(isLastInGroup(msgs, 0)).toBe(false)
  })

  it("is true when same sender but gap exceeds 5 minutes", () => {
    const msgs = [
      msg("1", "alice", "2024-06-15T12:00:00Z"),
      msg("2", "alice", "2024-06-15T12:06:00Z"),
    ]
    expect(isLastInGroup(msgs, 0)).toBe(true)
  })

  it("is true when messages span a day boundary", () => {
    const d1 = new Date(2024, 5, 15, 23, 59, 0)
    const d2 = new Date(2024, 5, 16, 0, 1, 0)
    const msgs = [msg("1", "alice", d1.toISOString()), msg("2", "alice", d2.toISOString())]
    expect(isLastInGroup(msgs, 0)).toBe(true)
  })

  it("is true when the current message is a tombstone", () => {
    const msgs = [
      msg("1", "alice", "2024-06-15T12:00:00Z", { tombstone: true }),
      msg("2", "alice", "2024-06-15T12:01:00Z"),
    ]
    expect(isLastInGroup(msgs, 0)).toBe(true)
  })
})

// ─── formatExpiry ───────────────────────────────────────────────────────────

describe("formatExpiry", () => {
  const now = Date.now()

  it("returns 'expired' when diff is zero", () => {
    expect(formatExpiry(new Date(now).toISOString(), now)).toBe("expired")
  })

  it("returns 'expired' when diff is negative", () => {
    expect(formatExpiry(new Date(now - 1000).toISOString(), now)).toBe("expired")
  })

  it("returns '< 1m' when less than 1 minute remains", () => {
    expect(formatExpiry(new Date(now + 30_000).toISOString(), now)).toBe("< 1m")
  })

  it("returns minutes when less than 1 hour remains", () => {
    expect(formatExpiry(new Date(now + 30 * 60_000).toISOString(), now)).toBe("30m left")
  })

  it("returns hours when less than 24 hours remain", () => {
    expect(formatExpiry(new Date(now + 3 * 3_600_000).toISOString(), now)).toBe("3h left")
  })

  it("returns days when 24+ hours remain", () => {
    expect(formatExpiry(new Date(now + 2 * 24 * 3_600_000).toISOString(), now)).toBe("2d left")
  })
})

// ─── expiryColorClass ───────────────────────────────────────────────────────

describe("expiryColorClass", () => {
  const now = Date.now()

  it("returns red for < 10 minutes", () => {
    expect(expiryColorClass(new Date(now + 5 * 60_000).toISOString(), now)).toBe("text-red-400")
  })

  it("returns amber for 10–59 minutes", () => {
    expect(expiryColorClass(new Date(now + 30 * 60_000).toISOString(), now)).toBe("text-amber-400")
  })

  it("returns gray for 60+ minutes", () => {
    expect(expiryColorClass(new Date(now + 2 * 3_600_000).toISOString(), now)).toBe("text-gray-500")
  })
})
