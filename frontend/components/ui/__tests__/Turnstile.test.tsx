import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, cleanup } from "@testing-library/react"

describe("Turnstile", () => {
  beforeEach(() => {
    // Ensure SITE_KEY is present so the component actually renders the widget
    process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY = "test-site"
    // Provide a fake global turnstile API
    window.turnstile = {
      render: vi.fn(() => "wid-1"),
      remove: vi.fn(),
    }
  })

  afterEach(() => {
    cleanup()
    delete window.turnstile
    delete process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY
  })

  it("calls render on mount and remove+render on reset", async () => {
    const Turnstile = (await import("@/components/ui/Turnstile")).default
    const onVerify = vi.fn()
    const { rerender, unmount } = render(<Turnstile onVerify={onVerify} />)

    expect(window.turnstile).toBeDefined()
    expect(window.turnstile?.render).toHaveBeenCalledTimes(1)

    // Simulate resetTrigger change
    rerender(<Turnstile onVerify={onVerify} resetTrigger={1} />)
    // remove should be called for previous widget id and render called again
    expect(window.turnstile?.remove).toHaveBeenCalled()
    expect(window.turnstile?.render).toHaveBeenCalled()

    // Unmount should clean up
    unmount()
    expect(window.turnstile?.remove).toHaveBeenCalled()
  })
})
