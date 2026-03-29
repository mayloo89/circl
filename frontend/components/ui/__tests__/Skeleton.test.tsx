import { describe, it, expect } from "vitest"
import { render } from "@testing-library/react"
import Skeleton from "@/components/ui/Skeleton"

describe("Skeleton", () => {
  it("renders a span with animate-pulse", () => {
    const { container } = render(<Skeleton />)
    expect(container.firstChild).toHaveClass("animate-pulse")
  })

  it("applies additional className", () => {
    const { container } = render(<Skeleton className="h-4 w-32" />)
    expect(container.firstChild).toHaveClass("h-4", "w-32")
  })
})
