import { describe, it, expect, vi, afterEach } from "vitest"
import { renderHook, waitFor, act } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { usePresence, formatLastSeen } from "@/hooks/usePresence"
import type { ContactEvent } from "@/hooks/useNotifications"

const API = "http://localhost:8080"

const presenceData = [
  { user_id: "user-1", online: true, last_seen_at: null },
  { user_id: "user-2", online: false, last_seen_at: "2024-06-15T12:00:00Z" },
]

afterEach(() => vi.restoreAllMocks())

describe("usePresence", () => {
  it("returns an empty map when no token is provided", () => {
    const { result } = renderHook(() => usePresence(["user-1"], undefined))
    expect(result.current).toEqual({})
  })

  it("returns an empty map when userIDs is empty", () => {
    const { result } = renderHook(() => usePresence([], "token"))
    expect(result.current).toEqual({})
  })

  it("fetches presence and builds a keyed map", async () => {
    server.use(http.get(`${API}/presence`, () => HttpResponse.json(presenceData)))
    const { result } = renderHook(() => usePresence(["user-1", "user-2"], "token"))
    await waitFor(() => expect(result.current["user-1"]).toBeDefined())
    expect(result.current["user-1"]?.online).toBe(true)
    expect(result.current["user-2"]?.online).toBe(false)
  })

  it("sets up the 30-second polling interval", () => {
    server.use(http.get(`${API}/presence`, () => HttpResponse.json(presenceData)))
    const setIntervalSpy = vi.spyOn(globalThis, "setInterval")
    renderHook(() => usePresence(["user-1"], "token"))
    expect(setIntervalSpy).toHaveBeenCalledWith(expect.any(Function), 30_000)
  })

  it("handles a non-ok response without crashing", async () => {
    server.use(http.get(`${API}/presence`, () => new HttpResponse(null, { status: 500 })))
    const { result } = renderHook(() => usePresence(["user-1"], "token"))
    // Give effects time to run — state should stay at the initial {}
    await new Promise((r) => setTimeout(r, 50))
    expect(result.current).toEqual({})
  })

  it("updates a user to online via SSE subscribe", async () => {
    server.use(
      http.get(`${API}/presence`, () =>
        HttpResponse.json([{ user_id: "user-1", online: false, last_seen_at: null }])
      )
    )
    let listener: ((e: ContactEvent) => void) | null = null
    const subscribe = vi.fn((fn: (e: ContactEvent) => void) => {
      listener = fn
      return () => { listener = null }
    })

    const { result } = renderHook(() => usePresence(["user-1"], "token", subscribe))
    await waitFor(() => expect(result.current["user-1"]).toBeDefined())
    expect(result.current["user-1"]?.online).toBe(false)

    act(() => {
      listener?.({ type: "presence_online", payload: { user_id: "user-1" } } as ContactEvent)
    })
    expect(result.current["user-1"]?.online).toBe(true)
  })

  it("updates a user to offline via SSE subscribe", async () => {
    server.use(
      http.get(`${API}/presence`, () =>
        HttpResponse.json([{ user_id: "user-1", online: true, last_seen_at: null }])
      )
    )
    let listener: ((e: ContactEvent) => void) | null = null
    const subscribe = vi.fn((fn: (e: ContactEvent) => void) => {
      listener = fn
      return () => { listener = null }
    })

    const { result } = renderHook(() => usePresence(["user-1"], "token", subscribe))
    await waitFor(() => expect(result.current["user-1"]).toBeDefined())
    expect(result.current["user-1"]?.online).toBe(true)

    act(() => {
      listener?.({ type: "presence_offline", payload: { user_id: "user-1" } } as ContactEvent)
    })
    expect(result.current["user-1"]?.online).toBe(false)
  })

  it("ignores presence_offline SSE event for an unknown user", async () => {
    server.use(http.get(`${API}/presence`, () => HttpResponse.json(presenceData)))
    let listener: ((e: ContactEvent) => void) | null = null
    const subscribe = vi.fn((fn: (e: ContactEvent) => void) => {
      listener = fn
      return () => {}
    })
    const { result } = renderHook(() => usePresence(["user-1", "user-2"], "token", subscribe))
    await waitFor(() => expect(result.current["user-1"]).toBeDefined())

    const before = result.current
    act(() => {
      listener?.({ type: "presence_offline", payload: { user_id: "unknown" } } as ContactEvent)
    })
    expect(result.current).toBe(before)
  })

  it("ignores SSE events for unknown users", async () => {
    server.use(http.get(`${API}/presence`, () => HttpResponse.json(presenceData)))
    let listener: ((e: ContactEvent) => void) | null = null
    const subscribe = vi.fn((fn: (e: ContactEvent) => void) => {
      listener = fn
      return () => {}
    })
    const { result } = renderHook(() => usePresence(["user-1", "user-2"], "token", subscribe))
    await waitFor(() => expect(result.current["user-1"]).toBeDefined())

    const before = result.current
    act(() => {
      listener?.({ type: "presence_online", payload: { user_id: "unknown" } } as ContactEvent)
    })
    expect(result.current).toBe(before)
  })
})

// ─── formatLastSeen ─────────────────────────────────────────────────────────

describe("formatLastSeen", () => {
  it("returns 'a while ago' when lastSeenAt is null", () => {
    expect(formatLastSeen(null)).toBe("a while ago")
  })

  it("returns 'just now' for less than 1 minute ago", () => {
    const t = new Date(Date.now() - 30_000).toISOString()
    expect(formatLastSeen(t)).toBe("just now")
  })

  it("returns '1 minute ago' (singular)", () => {
    const t = new Date(Date.now() - 90_000).toISOString()
    expect(formatLastSeen(t)).toBe("1 minute ago")
  })

  it("returns 'N minutes ago' for 2–59 minutes", () => {
    const t = new Date(Date.now() - 5 * 60_000).toISOString()
    expect(formatLastSeen(t)).toBe("5 minutes ago")
  })

  it("returns '1 hour ago' (singular)", () => {
    const t = new Date(Date.now() - 90 * 60_000).toISOString()
    expect(formatLastSeen(t)).toBe("1 hour ago")
  })

  it("returns 'N hours ago' for 2–23 hours", () => {
    const t = new Date(Date.now() - 5 * 3_600_000).toISOString()
    expect(formatLastSeen(t)).toBe("5 hours ago")
  })

  it("returns 'yesterday' for exactly 1 day ago", () => {
    const t = new Date(Date.now() - 24 * 3_600_000).toISOString()
    expect(formatLastSeen(t)).toBe("yesterday")
  })

  it("returns 'N days ago' for 2+ days", () => {
    const t = new Date(Date.now() - 3 * 24 * 3_600_000).toISOString()
    expect(formatLastSeen(t)).toBe("3 days ago")
  })
})
