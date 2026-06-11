import { describe, it, expect, vi } from "vitest"
import { useRef, useState } from "react"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { useFocusTrap, type UseFocusTrapOptions } from "@/hooks/useFocusTrap"

function Harness({
  onEscape,
  lockBodyScroll,
  children,
}: Pick<UseFocusTrapOptions, "onEscape" | "lockBodyScroll"> & { children?: React.ReactNode }) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  useFocusTrap({ active: open, containerRef: ref, onEscape, lockBodyScroll })
  return (
    <div>
      <button onClick={() => setOpen(true)}>Open dialog</button>
      {open && (
        <div ref={ref} role="dialog" aria-label="Trap">
          {children ?? (
            <>
              <button onClick={() => setOpen(false)}>First</button>
              <button>Middle</button>
              <button>Last</button>
            </>
          )}
        </div>
      )}
    </div>
  )
}

describe("useFocusTrap", () => {
  it("auto-focuses the first focusable child on activation", async () => {
    render(<Harness />)
    fireEvent.click(screen.getByText("Open dialog"))
    await waitFor(() =>
      expect(document.activeElement).toBe(screen.getByText("First")),
    )
  })

  it("wraps Tab from the last element back to the first", async () => {
    render(<Harness />)
    fireEvent.click(screen.getByText("Open dialog"))
    await waitFor(() => expect(document.activeElement).toBe(screen.getByText("First")))

    screen.getByText("Last").focus()
    fireEvent.keyDown(document, { key: "Tab" })
    expect(document.activeElement).toBe(screen.getByText("First"))
  })

  it("wraps Shift+Tab from the first element to the last", async () => {
    render(<Harness />)
    fireEvent.click(screen.getByText("Open dialog"))
    await waitFor(() => expect(document.activeElement).toBe(screen.getByText("First")))

    fireEvent.keyDown(document, { key: "Tab", shiftKey: true })
    expect(document.activeElement).toBe(screen.getByText("Last"))
  })

  it("pulls focus back inside when it escaped the container", async () => {
    render(<Harness />)
    fireEvent.click(screen.getByText("Open dialog"))
    await waitFor(() => expect(document.activeElement).toBe(screen.getByText("First")))

    // Simulate focus leaking to background content.
    screen.getByText("Open dialog").focus()
    fireEvent.keyDown(document, { key: "Tab" })
    expect(document.activeElement).toBe(screen.getByText("First"))
  })

  it("calls onEscape on Escape", () => {
    const onEscape = vi.fn()
    render(<Harness onEscape={onEscape} />)
    fireEvent.click(screen.getByText("Open dialog"))
    fireEvent.keyDown(document, { key: "Escape" })
    expect(onEscape).toHaveBeenCalledOnce()
  })

  it("ignores Escape when no handler is given", () => {
    render(<Harness />)
    fireEvent.click(screen.getByText("Open dialog"))
    fireEvent.keyDown(document, { key: "Escape" })
    expect(screen.getByRole("dialog")).toBeInTheDocument()
  })

  it("restores focus to the trigger when the trap deactivates", async () => {
    render(<Harness />)
    const trigger = screen.getByText("Open dialog")
    trigger.focus()
    fireEvent.click(trigger)
    await waitFor(() => expect(document.activeElement).toBe(screen.getByText("First")))

    // The First button closes the dialog.
    fireEvent.click(screen.getByText("First"))
    expect(document.activeElement).toBe(trigger)
  })

  it("locks body scroll while active and restores it on close", () => {
    render(<Harness />)
    fireEvent.click(screen.getByText("Open dialog"))
    expect(document.body.style.overflow).toBe("hidden")
    fireEvent.click(screen.getByText("First"))
    expect(document.body.style.overflow).toBe("")
  })

  it("leaves body scroll alone when lockBodyScroll is false", () => {
    render(<Harness lockBodyScroll={false} />)
    fireEvent.click(screen.getByText("Open dialog"))
    expect(document.body.style.overflow).toBe("")
  })

  it("keeps Tab from leaking when the container has no focusable children", async () => {
    render(
      <Harness>
        <p>Static content only</p>
      </Harness>,
    )
    fireEvent.click(screen.getByText("Open dialog"))
    const event = new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true })
    document.dispatchEvent(event)
    expect(event.defaultPrevented).toBe(true)
  })
})
