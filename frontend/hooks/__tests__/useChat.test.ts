import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { renderHook, act, waitFor } from "@testing-library/react"
import { FakeWebSocket } from "@/test/fakeSockets"
import { useChat } from "@/hooks/useChat"

const API = "http://localhost:8080"

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  })
}

let fetchMock: ReturnType<typeof vi.fn>

async function renderConnected() {
  const view = renderHook(() => useChat("room-1", "jwt-token"))
  await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1))
  const ws = FakeWebSocket.instances[0]
  act(() => ws.open())
  expect(view.result.current.connected).toBe(true)
  return { ...view, ws }
}

describe("useChat", () => {
  beforeEach(() => {
    FakeWebSocket.reset()
    vi.stubGlobal("WebSocket", FakeWebSocket)
    fetchMock = vi.fn().mockResolvedValue(jsonResponse({ ticket: "tkt-1" }))
    vi.stubGlobal("fetch", fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("does not connect without a room or token", async () => {
    renderHook(() => useChat(null, "jwt-token"))
    renderHook(() => useChat("room-1", undefined))
    await act(async () => {})
    expect(fetchMock).not.toHaveBeenCalled()
    expect(FakeWebSocket.instances).toHaveLength(0)
  })

  it("exchanges the JWT for a single-use ticket and never puts the JWT in the WS URL", async () => {
    const { ws } = await renderConnected()
    expect(fetchMock).toHaveBeenCalledWith(`${API}/ws-ticket`, {
      method: "POST",
      headers: { Authorization: "Bearer jwt-token" },
    })
    expect(ws.url).toBe("ws://localhost:8080/chat/rooms/room-1/ws?ticket=tkt-1")
    expect(ws.url).not.toContain("jwt-token")
  })

  it("sends messages with optional view-once and TTL flags", async () => {
    const { result, ws } = await renderConnected()
    act(() => {
      result.current.send("hola")
      result.current.send("secreto", { viewOnce: true, ttl: "15m" })
      result.current.sendTyping()
      result.current.sendAttachment("up-1", "http://cdn/a.jpg", "image/jpeg")
    })
    expect(ws.sent.map((s) => JSON.parse(s))).toEqual([
      { type: "message", content: "hola" },
      { type: "message", content: "secreto", view_once: true, ttl: "15m" },
      { type: "typing" },
      { type: "attachment", content: "http://cdn/a.jpg", mime_type: "image/jpeg", upload_id: "up-1" },
    ])
  })

  it("drops sends while the socket is not open", async () => {
    const view = renderHook(() => useChat("room-1", "jwt-token"))
    await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1))
    const ws = FakeWebSocket.instances[0]
    act(() => view.result.current.send("too early"))
    expect(ws.sent).toHaveLength(0)
  })

  it("appends incoming chat messages and ignores unknown frame types", async () => {
    const { result, ws } = await renderConnected()
    act(() => {
      ws.message({ type: "text", id: "m1", content: "hey" })
      ws.message({ type: "system", id: "m2", content: "note" })
      ws.message({ type: "bogus", id: "m3" })
      ws.message("not-json{")
    })
    expect(result.current.messages.map((m) => m.id)).toEqual(["m1", "m2"])
  })

  it("tracks deleted ids without removing the message", async () => {
    const { result, ws } = await renderConnected()
    act(() => {
      ws.message({ type: "text", id: "m1", content: "hey" })
      ws.message({ event: "message_deleted", id: "m1" })
    })
    expect(result.current.messages).toHaveLength(1)
    expect(result.current.deletedIds.has("m1")).toBe(true)
  })

  it("records read receipts and skips invalid timestamps", async () => {
    const { result, ws } = await renderConnected()
    act(() => {
      ws.message({ event: "read_receipt", user_id: "u2", read_at: "2026-06-10T12:00:00Z" })
      ws.message({ event: "read_receipt", user_id: "u3", read_at: "not a date" })
    })
    expect(result.current.readReceipts.get("u2")).toBe(Date.parse("2026-06-10T12:00:00Z"))
    expect(result.current.readReceipts.has("u3")).toBe(false)
  })

  it("tracks typing users", async () => {
    const { result, ws } = await renderConnected()
    act(() => ws.message({ event: "typing", user_id: "u2", display_name: "Bea" }))
    expect(result.current.typingUsers.get("u2")?.displayName).toBe("Bea")
  })

  it("accumulates participant join and leave events", async () => {
    const { result, ws } = await renderConnected()
    act(() => {
      ws.message({ event: "participant_join", user_id: "u2", display_name: "Bea", is_guest: true })
      ws.message({ event: "participant_leave", user_id: "u2", is_guest: true })
    })
    expect(result.current.participantEvents).toEqual([
      { type: "join", userId: "u2", username: "", displayName: "Bea", avatarURL: "", isGuest: true },
      { type: "leave", userId: "u2", username: "", displayName: "", avatarURL: "", isGuest: true },
    ])
  })

  it("closes for good when kicked — no reconnect", async () => {
    const { result, ws } = await renderConnected()
    act(() => ws.message({ event: "kicked" }))
    expect(result.current.isKicked).toBe(true)
    expect(ws.readyState).toBe(FakeWebSocket.CLOSED)
    // A reconnect would create a second socket; being cancelled, it must not.
    await act(async () => {})
    expect(FakeWebSocket.instances).toHaveLength(1)
  })

  it("toggles mute state from server events", async () => {
    const { result, ws } = await renderConnected()
    act(() => ws.message({ event: "you_are_muted" }))
    expect(result.current.isMuted).toBe(true)
    act(() => ws.message({ event: "you_are_unmuted" }))
    expect(result.current.isMuted).toBe(false)
  })

  it("closes the socket and resets state on unmount", async () => {
    const { ws, unmount } = await renderConnected()
    unmount()
    expect(ws.readyState).toBe(FakeWebSocket.CLOSED)
    expect(FakeWebSocket.instances).toHaveLength(1)
  })

  it("retries with backoff when the ticket endpoint fails", async () => {
    vi.useFakeTimers()
    try {
      fetchMock
        .mockResolvedValueOnce(jsonResponse({ error: "boom" }, 500))
        .mockResolvedValue(jsonResponse({ ticket: "tkt-2" }))
      renderHook(() => useChat("room-1", "jwt-token"))
      await act(async () => {
        await vi.advanceTimersByTimeAsync(0)
      })
      expect(fetchMock).toHaveBeenCalledTimes(1)
      expect(FakeWebSocket.instances).toHaveLength(0)
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1000)
      })
      expect(fetchMock).toHaveBeenCalledTimes(2)
      expect(FakeWebSocket.instances).toHaveLength(1)
    } finally {
      vi.useRealTimers()
    }
  })
})
