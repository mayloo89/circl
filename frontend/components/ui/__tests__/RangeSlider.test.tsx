import { describe, it, expect, vi } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import RangeSlider from "@/components/ui/RangeSlider"

describe("RangeSlider", () => {
  it("exposes the accessible label and formatted value text", () => {
    render(
      <RangeSlider
        min={0}
        max={500}
        value={500}
        onChange={() => {}}
        ariaLabel="Maximum distance"
        formatValue={(v) => (v === 500 ? "∞ km" : `${v} km`)}
      />,
    )
    const slider = screen.getByRole("slider", { name: "Maximum distance" })
    expect(slider).toHaveAttribute("aria-valuetext", "∞ km")
    expect(screen.getByText("∞ km")).toBeInTheDocument()
  })

  it("falls back to the raw number when no formatter is given", () => {
    render(<RangeSlider min={18} max={99} value={25} onChange={() => {}} ariaLabel="Minimum age" />)
    expect(screen.getByText("25")).toBeInTheDocument()
    expect(screen.getByRole("slider")).toHaveAttribute("aria-valuetext", "25")
  })

  it("reports changes as numbers", () => {
    const onChange = vi.fn()
    render(<RangeSlider min={18} max={99} value={25} onChange={onChange} ariaLabel="Minimum age" />)
    fireEvent.change(screen.getByRole("slider"), { target: { value: "42" } })
    expect(onChange).toHaveBeenCalledWith(42)
  })

  it("disables the input when disabled", () => {
    render(<RangeSlider min={0} max={10} value={5} onChange={() => {}} disabled ariaLabel="X" />)
    expect(screen.getByRole("slider")).toBeDisabled()
  })
})
