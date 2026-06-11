import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent, waitFor } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"
import BottomSheet from "@/components/ui/BottomSheet"

describe("BottomSheet", () => {
  it("renders nothing when closed", () => {
    renderWithIntl(
      <BottomSheet open={false} onClose={vi.fn()} title="Filters">
        <p>Body</p>
      </BottomSheet>,
    )
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
  })

  it("renders a labeled dialog with its children when open", () => {
    renderWithIntl(
      <BottomSheet open onClose={vi.fn()} title="Filters">
        <p>Body</p>
      </BottomSheet>,
    )
    expect(screen.getByRole("dialog", { name: "Filters" })).toBeInTheDocument()
    expect(screen.getByText("Body")).toBeInTheDocument()
  })

  it("closes via the close button", () => {
    const onClose = vi.fn()
    renderWithIntl(
      <BottomSheet open onClose={onClose} title="Filters">
        <p>Body</p>
      </BottomSheet>,
    )
    fireEvent.click(screen.getByRole("button", { name: "Close" }))
    expect(onClose).toHaveBeenCalledOnce()
  })

  it("closes on Escape through the focus trap", () => {
    const onClose = vi.fn()
    renderWithIntl(
      <BottomSheet open onClose={onClose} title="Filters">
        <p>Body</p>
      </BottomSheet>,
    )
    fireEvent.keyDown(document, { key: "Escape" })
    expect(onClose).toHaveBeenCalledOnce()
  })

  it("auto-focuses inside the sheet when opened", async () => {
    renderWithIntl(
      <BottomSheet open onClose={vi.fn()} title="Filters">
        <button>First action</button>
      </BottomSheet>,
    )
    const dialog = screen.getByRole("dialog", { name: "Filters" })
    // Focus lands one animation frame after activation.
    await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true))
  })
})
