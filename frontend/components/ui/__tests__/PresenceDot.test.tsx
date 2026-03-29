import { describe, it, expect } from "vitest"
import { render } from "@testing-library/react"
import PresenceDot from "@/components/ui/PresenceDot"

describe("PresenceDot", () => {
  it("renders green when online", () => {
    const { container } = render(<PresenceDot online />)
    expect(container.firstChild).toHaveClass("bg-green-400")
  })

  it("renders gray when offline", () => {
    const { container } = render(<PresenceDot online={false} />)
    expect(container.firstChild).toHaveClass("bg-gray-600")
  })

  it("applies md size by default", () => {
    const { container } = render(<PresenceDot online />)
    expect(container.firstChild).toHaveClass("h-2.5", "w-2.5")
  })

  it("applies sm size when specified", () => {
    const { container } = render(<PresenceDot online size="sm" />)
    expect(container.firstChild).toHaveClass("h-1.5", "w-1.5")
  })

  it("applies a custom className", () => {
    const { container } = render(<PresenceDot online className="absolute" />)
    expect(container.firstChild).toHaveClass("absolute")
  })
})
