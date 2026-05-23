import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, act, renderHook } from "@testing-library/react"
import { ToastProvider, useToast } from "@/components/ui/Toast"

// Helper component that exposes the toast function to tests
function TestConsumer({ onReady }: { onReady: (fn: ReturnType<typeof useToast>["toast"]) => void }) {
  const { toast } = useToast()
  onReady(toast)
  return null
}

function renderWithProvider() {
  let toastFn!: ReturnType<typeof useToast>["toast"]
  render(
    <ToastProvider>
      <TestConsumer onReady={(fn) => { toastFn = fn }} />
    </ToastProvider>
  )
  return { toast: toastFn }
}

describe("ToastProvider", () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  it("renders children", () => {
    render(
      <ToastProvider>
        <p>Hello</p>
      </ToastProvider>
    )
    expect(screen.getByText("Hello")).toBeInTheDocument()
  })

  it("shows a toast message when toast() is called", () => {
    const { toast } = renderWithProvider()
    act(() => { toast("Upload complete", "success") })
    expect(screen.getByText("Upload complete")).toBeInTheDocument()
  })

  it("shows an error toast", () => {
    const { toast } = renderWithProvider()
    act(() => { toast("Something went wrong", "error") })
    const el = screen.getByText("Something went wrong")
    expect(el.closest("div")).toHaveClass("bg-red-900")
  })

  it("shows a success toast", () => {
    const { toast } = renderWithProvider()
    act(() => { toast("Saved!", "success") })
    const el = screen.getByText("Saved!")
    expect(el.closest("div")).toHaveClass("bg-green-800")
  })

  it("shows an info toast by default", () => {
    const { toast } = renderWithProvider()
    act(() => { toast("Note") })
    const el = screen.getByText("Note")
    expect(el.closest("div")).toHaveClass("bg-gray-200")
  })

  it("auto-dismisses the toast after 4 seconds", () => {
    const { toast } = renderWithProvider()
    act(() => { toast("Temporary") })
    expect(screen.getByText("Temporary")).toBeInTheDocument()
    act(() => { vi.advanceTimersByTime(4_000) })
    expect(screen.queryByText("Temporary")).not.toBeInTheDocument()
  })

  it("can show multiple toasts", () => {
    const { toast } = renderWithProvider()
    act(() => {
      toast("First")
      toast("Second")
    })
    expect(screen.getByText("First")).toBeInTheDocument()
    expect(screen.getByText("Second")).toBeInTheDocument()
  })
})

describe("useToast outside provider", () => {
  it("returns a no-op toast function that does not throw", () => {
    const { result } = renderHook(() => useToast())
    expect(() => result.current.toast("noop")).not.toThrow()
  })
})
