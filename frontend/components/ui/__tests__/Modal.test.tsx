import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import Modal from "@/components/ui/Modal"

describe("Modal", () => {
  it("renders nothing when open is false", () => {
    render(
      <Modal open={false} onClose={vi.fn()}>
        <p>Hidden content</p>
      </Modal>
    )
    expect(screen.queryByText("Hidden content")).not.toBeInTheDocument()
  })

  it("renders children when open is true", () => {
    render(
      <Modal open onClose={vi.fn()}>
        <p>Visible content</p>
      </Modal>
    )
    expect(screen.getByText("Visible content")).toBeInTheDocument()
  })

  it("calls onClose when Escape key is pressed", () => {
    const onClose = vi.fn()
    render(
      <Modal open onClose={onClose}>
        <p>Content</p>
      </Modal>
    )
    fireEvent.keyDown(document, { key: "Escape" })
    expect(onClose).toHaveBeenCalledOnce()
  })

  it("calls onClose when the backdrop is clicked", () => {
    const onClose = vi.fn()
    const { container } = render(
      <Modal open onClose={onClose}>
        <p>Content</p>
      </Modal>
    )
    // Click the backdrop div (the fixed overlay)
    fireEvent.click(container.firstChild!)
    expect(onClose).toHaveBeenCalledOnce()
  })

  it("does not call onClose for non-Escape keys", () => {
    const onClose = vi.fn()
    render(
      <Modal open onClose={onClose}>
        <p>Content</p>
      </Modal>
    )
    fireEvent.keyDown(document, { key: "Enter" })
    fireEvent.keyDown(document, { key: "Tab" })
    expect(onClose).not.toHaveBeenCalled()
  })

  it("does not register Escape listener when closed", () => {
    const onClose = vi.fn()
    render(
      <Modal open={false} onClose={onClose}>
        <p>Content</p>
      </Modal>
    )
    fireEvent.keyDown(document, { key: "Escape" })
    expect(onClose).not.toHaveBeenCalled()
  })
})
