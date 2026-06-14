import { describe, it, expect, vi, beforeEach } from "vitest"
import { render } from "@testing-library/react"
import ClientErrorReporter from "@/components/ClientErrorReporter"
import { reportClientError } from "@/lib/reportError"

vi.mock("@/lib/reportError", () => ({ reportClientError: vi.fn() }))

const reported = vi.mocked(reportClientError)

describe("ClientErrorReporter", () => {
  beforeEach(() => reported.mockClear())

  it("reports uncaught window errors", () => {
    render(<ClientErrorReporter />)
    const error = new Error("boom")
    window.dispatchEvent(Object.assign(new Event("error"), { error }))
    expect(reported).toHaveBeenCalledWith({ error, kind: "error" })
  })

  it("reports unhandled promise rejections", () => {
    render(<ClientErrorReporter />)
    window.dispatchEvent(Object.assign(new Event("unhandledrejection"), { reason: "nope" }))
    expect(reported).toHaveBeenCalledWith({ error: "nope", kind: "unhandledrejection" })
  })

  it("removes its listeners on unmount", () => {
    const removeSpy = vi.spyOn(window, "removeEventListener")
    const { unmount } = render(<ClientErrorReporter />)
    unmount()
    expect(removeSpy).toHaveBeenCalledWith("error", expect.any(Function))
    expect(removeSpy).toHaveBeenCalledWith("unhandledrejection", expect.any(Function))
    removeSpy.mockRestore()
  })
})
