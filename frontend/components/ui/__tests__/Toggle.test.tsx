import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import Toggle from "@/components/ui/Toggle"

describe("Toggle", () => {
  it("renders a switch labeled by its visible text", () => {
    render(<Toggle label="Hide my distance" checked={false} onChange={() => {}} />)
    const input = screen.getByRole("switch", { name: "Hide my distance" })
    expect(input).toHaveAttribute("aria-checked", "false")
    expect(input).not.toBeChecked()
  })

  it("reflects the checked state", () => {
    render(<Toggle label="Hide my distance" checked onChange={() => {}} />)
    const input = screen.getByRole("switch")
    expect(input).toHaveAttribute("aria-checked", "true")
    expect(input).toBeChecked()
  })

  it("shows description and badge when provided", () => {
    render(
      <Toggle
        label="Read receipts"
        description="Applies in both directions."
        badge={<span>Symmetric</span>}
        checked={false}
        onChange={() => {}}
      />,
    )
    expect(screen.getByText("Applies in both directions.")).toBeInTheDocument()
    expect(screen.getByText("Symmetric")).toBeInTheDocument()
  })

  it("fires onChange when clicked", () => {
    const onChange = vi.fn()
    render(<Toggle label="Read receipts" checked={false} onChange={onChange} />)
    fireEvent.click(screen.getByRole("switch"))
    expect(onChange).toHaveBeenCalledOnce()
  })

  it("is disabled when the disabled prop is set", () => {
    render(<Toggle label="Read receipts" checked={false} disabled onChange={() => {}} />)
    expect(screen.getByRole("switch")).toBeDisabled()
  })

  it("derives a stable input id from the label for the wrapping label element", () => {
    render(<Toggle label="Hide my distance" checked={false} onChange={() => {}} />)
    expect(screen.getByRole("switch")).toHaveAttribute("id", "toggle-hide-my-distance")
  })
})
