import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import Badge from "@/components/ui/Badge"

describe("Badge", () => {
  describe("count variant (default)", () => {
    it("shows the count", () => {
      render(<Badge count={5} />)
      expect(screen.getByText("5")).toBeInTheDocument()
    })

    it("shows max+ when count exceeds max", () => {
      render(<Badge count={15} max={9} />)
      expect(screen.getByText("9+")).toBeInTheDocument()
    })

    it("shows exact count when equal to max", () => {
      render(<Badge count={9} max={9} />)
      expect(screen.getByText("9")).toBeInTheDocument()
    })

    it("shows count without max cap when max is not set", () => {
      render(<Badge count={100} />)
      expect(screen.getByText("100")).toBeInTheDocument()
    })
  })

  describe("dot variant", () => {
    it("renders in a dot-sized container", () => {
      const { container } = render(<Badge count={3} variant="dot" />)
      expect(container.firstChild).toHaveClass("h-4", "w-4")
    })

    it("applies max threshold", () => {
      render(<Badge count={10} max={9} variant="dot" />)
      expect(screen.getByText("9+")).toBeInTheDocument()
    })
  })

  describe("pill variant", () => {
    it("renders with horizontal padding", () => {
      const { container } = render(<Badge count={7} variant="pill" />)
      expect(container.firstChild).toHaveClass("px-2")
    })
  })

  it("applies a custom className", () => {
    const { container } = render(<Badge count={1} className="ml-2" />)
    expect(container.firstChild).toHaveClass("ml-2")
  })
})
