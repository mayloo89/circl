import { describe, it, expect, vi, afterEach } from "vitest"
import { renderHook, act } from "@testing-library/react"
import { usePush } from "@/hooks/usePush"

// jsdom exposes neither navigator.serviceWorker nor window.PushManager, so the
// hook runs its unsupported-environment path — the guard that production code
// relies on for Safari < 16 and private-mode browsers.
describe("usePush (unsupported environment)", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("reports unsupported and denied", () => {
    const { result } = renderHook(() => usePush("jwt-token"))
    expect(result.current.supported).toBe(false)
    expect(result.current.permission).toBe("denied")
  })

  it("enable and disable are safe no-ops", async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal("fetch", fetchMock)
    const { result } = renderHook(() => usePush("jwt-token"))
    await act(async () => {
      await result.current.enable()
      await result.current.disable()
    })
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
