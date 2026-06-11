import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { renderHook, act, waitFor } from "@testing-library/react"
import { FakeEventSource } from "@/test/fakeSockets"
import { useNotifications, type ContactEvent } from "@/hooks/useNotifications"

const API = "http://localhost:8080"

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  })
}

let fetchMock: ReturnType<typeof vi.fn>

describe("useNotifications", () => {
  beforeEach(() => {
    FakeEventSource.reset()
    vi.stubGlobal("EventSource", FakeEventSource)
    fetchMock = vi.fn().mockResolvedValue(jsonResponse({ ticket: "sse-1" }))
    vi.stubGlobal("fetch", fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("does nothing without a token", async () => {
    renderHook(() => useNotifications(undefined, vi.fn()))
    await act(async () => {})
    expect(fetchMock).not.toHaveBeenCalled()
    expect(FakeEventSource.instances).toHaveLength(0)
  })

  it("connects with a single-use ticket instead of the JWT", async () => {
    renderHook(() => useNotifications("jwt-token", vi.fn()))
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))
    expect(fetchMock).toHaveBeenCalledWith(`${API}/ws-ticket`, {
      method: "POST",
      headers: { Authorization: "Bearer jwt-token" },
    })
    const es = FakeEventSource.instances[0]
    expect(es.url).toBe(`${API}/notifications/stream?ticket=sse-1`)
    expect(es.url).not.toContain("jwt-token")
  })

  it("forwards parsed events and ignores malformed payloads", async () => {
    const onEvent = vi.fn()
    renderHook(() => useNotifications("jwt-token", onEvent))
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))
    const es = FakeEventSource.instances[0]
    act(() => {
      es.message({ type: "contact_request", payload: { contact_id: "c1", requester_id: "u2" } })
      es.message("{broken json")
      es.message({ type: "new_message", payload: { room_id: "r1" } })
    })
    expect(onEvent).toHaveBeenCalledTimes(2)
    expect(onEvent).toHaveBeenNthCalledWith(1, {
      type: "contact_request",
      payload: { contact_id: "c1", requester_id: "u2" },
    })
  })

  it("always dispatches to the latest callback after re-renders", async () => {
    const first = vi.fn()
    const second = vi.fn()
    const view = renderHook(
      ({ cb }: { cb: (e: ContactEvent) => void }) => useNotifications("jwt-token", cb),
      { initialProps: { cb: first } },
    )
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))
    view.rerender({ cb: second })
    act(() => FakeEventSource.instances[0].message({ type: "connected" }))
    expect(first).not.toHaveBeenCalled()
    expect(second).toHaveBeenCalledWith({ type: "connected" })
    // The connection must not have been re-created by the callback change.
    expect(FakeEventSource.instances).toHaveLength(1)
  })

  it("reconnects with a fresh ticket after a stream error", async () => {
    vi.useFakeTimers()
    try {
      fetchMock
        .mockResolvedValueOnce(jsonResponse({ ticket: "sse-1" }))
        .mockResolvedValue(jsonResponse({ ticket: "sse-2" }))
      renderHook(() => useNotifications("jwt-token", vi.fn()))
      await act(async () => {
        await vi.advanceTimersByTimeAsync(0)
      })
      expect(FakeEventSource.instances).toHaveLength(1)
      act(() => FakeEventSource.instances[0].error())
      expect(FakeEventSource.instances[0].closed).toBe(true)
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1000)
      })
      expect(FakeEventSource.instances).toHaveLength(2)
      expect(FakeEventSource.instances[1].url).toContain("ticket=sse-2")
    } finally {
      vi.useRealTimers()
    }
  })

  it("closes the stream on unmount", async () => {
    const view = renderHook(() => useNotifications("jwt-token", vi.fn()))
    await waitFor(() => expect(FakeEventSource.instances).toHaveLength(1))
    view.unmount()
    expect(FakeEventSource.instances[0].closed).toBe(true)
  })
})
