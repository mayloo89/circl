import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { renderHook, act, waitFor } from "@testing-library/react"
import { FakeWebSocket } from "@/test/fakeSockets"
import { useGuestChat } from "@/hooks/useGuestChat"

const API = "http://localhost:8080"

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  })
}

let fetchMock: ReturnType<typeof vi.fn>

describe("useGuestChat", () => {
  beforeEach(() => {
    FakeWebSocket.reset()
    vi.stubGlobal("WebSocket", FakeWebSocket)
    fetchMock = vi.fn().mockResolvedValue(jsonResponse({ ticket: "gt-1" }))
    vi.stubGlobal("fetch", fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("does not connect without a room or session", async () => {
    renderHook(() => useGuestChat(null, "sess-1"))
    renderHook(() => useGuestChat("room-1", undefined))
    await act(async () => {})
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it("requests a guest ticket with the session and room ids", async () => {
    renderHook(() => useGuestChat("room-1", "sess-1"))
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1))
    expect(fetchMock).toHaveBeenCalledWith(`${API}/guest/ws-ticket`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ session_id: "sess-1", room_id: "room-1" }),
    })
    expect(FakeWebSocket.instances[0].url).toBe(
      "ws://localhost:8080/chat/rooms/room-1/ws?ticket=gt-1",
    )
  })

  it("treats a 401 ticket response as an invalid session — no retry, no socket", async () => {
    vi.useFakeTimers()
    try {
      fetchMock.mockResolvedValue(jsonResponse({ error: "unauthorized" }, 401))
      const onInvalidSession = vi.fn()
      renderHook(() => useGuestChat("room-1", "sess-stale", onInvalidSession))
      await act(async () => {
        await vi.advanceTimersByTimeAsync(0)
      })
      expect(onInvalidSession).toHaveBeenCalledOnce()
      expect(FakeWebSocket.instances).toHaveLength(0)
      // A retry would re-hit the ticket endpoint; 401 must not.
      await act(async () => {
        await vi.advanceTimersByTimeAsync(60_000)
      })
      expect(fetchMock).toHaveBeenCalledTimes(1)
    } finally {
      vi.useRealTimers()
    }
  })

  it("retries on non-401 ticket failures", async () => {
    vi.useFakeTimers()
    try {
      fetchMock
        .mockResolvedValueOnce(jsonResponse({ error: "boom" }, 500))
        .mockResolvedValue(jsonResponse({ ticket: "gt-2" }))
      renderHook(() => useGuestChat("room-1", "sess-1"))
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1000)
      })
      expect(fetchMock).toHaveBeenCalledTimes(2)
      expect(FakeWebSocket.instances).toHaveLength(1)
    } finally {
      vi.useRealTimers()
    }
  })

  it("sends plain text messages only", async () => {
    const { result } = renderHook(() => useGuestChat("room-1", "sess-1"))
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1))
    const ws = FakeWebSocket.instances[0]
    act(() => ws.open())
    act(() => {
      result.current.send("hola")
      result.current.sendTyping()
    })
    expect(ws.sent.map((s) => JSON.parse(s))).toEqual([
      { type: "message", content: "hola" },
      { type: "typing" },
    ])
  })

  it("surfaces kick and mute moderation events", async () => {
    const { result } = renderHook(() => useGuestChat("room-1", "sess-1"))
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1))
    const ws = FakeWebSocket.instances[0]
    act(() => ws.open())
    act(() => ws.message({ event: "you_are_muted" }))
    expect(result.current.isMuted).toBe(true)
    act(() => ws.message({ event: "kicked" }))
    expect(result.current.isKicked).toBe(true)
    expect(ws.readyState).toBe(FakeWebSocket.CLOSED)
    await act(async () => {})
    expect(FakeWebSocket.instances).toHaveLength(1)
  })

  it("appends incoming messages from guests and registered users alike", async () => {
    const { result } = renderHook(() => useGuestChat("room-1", "sess-1"))
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1))
    const ws = FakeWebSocket.instances[0]
    act(() => ws.open())
    act(() => {
      ws.message({ type: "text", id: "m1", sender_name: "Guest77", content: "hi" })
      ws.message({ type: "text", id: "m2", sender_name: "Alice", content: "hello" })
    })
    expect(result.current.messages.map((m) => m.id)).toEqual(["m1", "m2"])
  })
})
