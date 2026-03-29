import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { renderHook, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { useHeartbeat } from "@/hooks/useHeartbeat"

const API = "http://localhost:8080"
const HEARTBEAT_URL = `${API}/presence/heartbeat`

beforeEach(() => {
  Object.defineProperty(document, "visibilityState", { writable: true, value: "visible" })
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe("useHeartbeat", () => {
  it("does nothing when token is undefined", async () => {
    let called = false
    server.use(http.post(HEARTBEAT_URL, () => { called = true; return new HttpResponse(null) }))
    renderHook(() => useHeartbeat(undefined))
    // Give React enough time to flush effects
    await new Promise((r) => setTimeout(r, 50))
    expect(called).toBe(false)
  })

  it("sends a heartbeat immediately on mount", async () => {
    let callCount = 0
    server.use(http.post(HEARTBEAT_URL, () => { callCount++; return new HttpResponse(null) }))
    renderHook(() => useHeartbeat("my-token"))
    await waitFor(() => expect(callCount).toBeGreaterThanOrEqual(1))
  })

  it("sets up the 20-second polling interval", () => {
    const setIntervalSpy = vi.spyOn(globalThis, "setInterval")
    server.use(http.post(HEARTBEAT_URL, () => new HttpResponse(null)))
    renderHook(() => useHeartbeat("my-token"))
    expect(setIntervalSpy).toHaveBeenCalledWith(expect.any(Function), 20_000)
  })

  it("does not send a heartbeat when the tab is hidden", async () => {
    Object.defineProperty(document, "visibilityState", { writable: true, value: "hidden" })
    let callCount = 0
    server.use(http.post(HEARTBEAT_URL, () => { callCount++; return new HttpResponse(null) }))
    renderHook(() => useHeartbeat("my-token"))
    await new Promise((r) => setTimeout(r, 50))
    expect(callCount).toBe(0)
  })

  it("ignores visibilitychange when the tab stays hidden", async () => {
    Object.defineProperty(document, "visibilityState", { writable: true, value: "hidden" })
    let callCount = 0
    server.use(http.post(HEARTBEAT_URL, () => { callCount++; return new HttpResponse(null) }))
    renderHook(() => useHeartbeat("my-token"))
    // Fire the event but keep state as hidden
    document.dispatchEvent(new Event("visibilitychange"))
    await new Promise((r) => setTimeout(r, 50))
    expect(callCount).toBe(0)
  })

  it("sends a heartbeat when the tab becomes visible", async () => {
    Object.defineProperty(document, "visibilityState", { writable: true, value: "hidden" })
    let callCount = 0
    server.use(http.post(HEARTBEAT_URL, () => { callCount++; return new HttpResponse(null) }))
    renderHook(() => useHeartbeat("my-token"))
    await new Promise((r) => setTimeout(r, 50))
    expect(callCount).toBe(0)

    Object.defineProperty(document, "visibilityState", { writable: true, value: "visible" })
    document.dispatchEvent(new Event("visibilitychange"))
    await waitFor(() => expect(callCount).toBe(1))
  })

  it("cleans up interval and event listener on unmount", () => {
    const clearIntervalSpy = vi.spyOn(globalThis, "clearInterval")
    const removeListenerSpy = vi.spyOn(document, "removeEventListener")
    server.use(http.post(HEARTBEAT_URL, () => new HttpResponse(null)))
    const { unmount } = renderHook(() => useHeartbeat("my-token"))
    unmount()
    expect(clearIntervalSpy).toHaveBeenCalled()
    expect(removeListenerSpy).toHaveBeenCalledWith("visibilitychange", expect.any(Function))
  })
})
