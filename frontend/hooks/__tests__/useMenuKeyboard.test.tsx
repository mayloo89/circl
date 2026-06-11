import { describe, it, expect, vi } from "vitest"
import { useRef, useState } from "react"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { useMenuKeyboard } from "@/hooks/useMenuKeyboard"

function MenuHarness({ onClose }: { onClose?: () => void }) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const close = () => {
    setOpen(false)
    onClose?.()
  }
  useMenuKeyboard({ open, containerRef: ref, onClose: close })
  return (
    <div>
      <button aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen(true)}>
        Actions
      </button>
      {open && (
        <div ref={ref} role="menu">
          <button role="menuitem">Block</button>
          <button role="menuitem">Report</button>
          <button role="menuitem">Unblock</button>
        </div>
      )}
    </div>
  )
}

function openMenu() {
  const trigger = screen.getByText("Actions")
  trigger.focus()
  fireEvent.click(trigger)
}

describe("useMenuKeyboard", () => {
  it("focuses the first menuitem when the menu opens", async () => {
    render(<MenuHarness />)
    openMenu()
    await waitFor(() => expect(document.activeElement).toBe(screen.getByText("Block")))
  })

  it("moves down with ArrowDown and wraps past the last item", async () => {
    render(<MenuHarness />)
    openMenu()
    await waitFor(() => expect(document.activeElement).toBe(screen.getByText("Block")))

    fireEvent.keyDown(document, { key: "ArrowDown" })
    expect(document.activeElement).toBe(screen.getByText("Report"))
    fireEvent.keyDown(document, { key: "ArrowDown" })
    expect(document.activeElement).toBe(screen.getByText("Unblock"))
    fireEvent.keyDown(document, { key: "ArrowDown" })
    expect(document.activeElement).toBe(screen.getByText("Block"))
  })

  it("moves up with ArrowUp and wraps before the first item", async () => {
    render(<MenuHarness />)
    openMenu()
    await waitFor(() => expect(document.activeElement).toBe(screen.getByText("Block")))

    fireEvent.keyDown(document, { key: "ArrowUp" })
    expect(document.activeElement).toBe(screen.getByText("Unblock"))
  })

  it("jumps to first and last with Home and End", async () => {
    render(<MenuHarness />)
    openMenu()
    await waitFor(() => expect(document.activeElement).toBe(screen.getByText("Block")))

    fireEvent.keyDown(document, { key: "End" })
    expect(document.activeElement).toBe(screen.getByText("Unblock"))
    fireEvent.keyDown(document, { key: "Home" })
    expect(document.activeElement).toBe(screen.getByText("Block"))
  })

  it("closes on Escape and restores focus to the trigger", async () => {
    const onClose = vi.fn()
    render(<MenuHarness onClose={onClose} />)
    openMenu()
    await waitFor(() => expect(document.activeElement).toBe(screen.getByText("Block")))

    fireEvent.keyDown(document, { key: "Escape" })
    expect(onClose).toHaveBeenCalledOnce()
    await waitFor(() => expect(document.activeElement).toBe(screen.getByText("Actions")))
  })

  it("closes on Tab without trapping focus", async () => {
    const onClose = vi.fn()
    render(<MenuHarness onClose={onClose} />)
    openMenu()
    await waitFor(() => expect(document.activeElement).toBe(screen.getByText("Block")))

    const event = new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true })
    document.dispatchEvent(event)
    expect(onClose).toHaveBeenCalledOnce()
    // Tab must NOT be prevented — focus continues past the menu.
    expect(event.defaultPrevented).toBe(false)
  })
})
