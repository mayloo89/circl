import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"
import ConfirmDialog from "@/components/ui/ConfirmDialog"

const baseProps = {
  open: true,
  title: "Remove contact",
  message: "This cannot be undone.",
  onConfirm: vi.fn(),
  onCancel: vi.fn(),
}

describe("ConfirmDialog", () => {
  it("renders nothing when closed", () => {
    renderWithIntl(<ConfirmDialog {...baseProps} open={false} />)
    expect(screen.queryByText("Remove contact")).not.toBeInTheDocument()
  })

  it("shows title, message, and the default confirm label", () => {
    renderWithIntl(<ConfirmDialog {...baseProps} />)
    expect(screen.getByText("Remove contact")).toBeInTheDocument()
    expect(screen.getByText("This cannot be undone.")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Confirm" })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument()
  })

  it("uses a custom confirm label when provided", () => {
    renderWithIntl(<ConfirmDialog {...baseProps} confirmLabel="Delete forever" />)
    expect(screen.getByRole("button", { name: "Delete forever" })).toBeInTheDocument()
  })

  it("invokes the matching callback for each button", () => {
    const onConfirm = vi.fn()
    const onCancel = vi.fn()
    renderWithIntl(<ConfirmDialog {...baseProps} onConfirm={onConfirm} onCancel={onCancel} />)
    fireEvent.click(screen.getByRole("button", { name: "Confirm" }))
    expect(onConfirm).toHaveBeenCalledOnce()
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }))
    expect(onCancel).toHaveBeenCalledOnce()
  })

  it("disables cancel while loading", () => {
    renderWithIntl(<ConfirmDialog {...baseProps} loading />)
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled()
  })
})
