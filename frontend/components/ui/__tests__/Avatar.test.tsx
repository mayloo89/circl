import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import Avatar from "@/components/ui/Avatar"

describe("Avatar", () => {
  it("renders an img when src is provided", () => {
    const { container } = render(<Avatar src="http://example.com/photo.jpg" name="Alice" />)
    expect(container.querySelector("img")).toBeInTheDocument()
  })

  it("renders initials when no src is provided", () => {
    render(<Avatar name="Bob" />)
    expect(screen.getByText("B")).toBeInTheDocument()
    expect(screen.queryByRole("img")).not.toBeInTheDocument()
  })

  it("uses the first character of name uppercased", () => {
    render(<Avatar name="charlie" />)
    expect(screen.getByText("C")).toBeInTheDocument()
  })

  it("shows '?' when name is empty", () => {
    render(<Avatar name="" />)
    expect(screen.getByText("?")).toBeInTheDocument()
  })

  it("applies indigo background when color is indigo", () => {
    const { container } = render(<Avatar name="Dave" color="indigo" />)
    expect(container.firstChild).toHaveClass("bg-indigo-700")
  })

  it("applies gray background by default", () => {
    const { container } = render(<Avatar name="Eve" />)
    expect(container.firstChild).toHaveClass("bg-gray-700")
  })

  it("applies the xl size class", () => {
    const { container } = render(<Avatar name="Frank" size="xl" />)
    expect(container.firstChild).toHaveClass("h-24", "w-24")
  })

  it("applies a custom className", () => {
    const { container } = render(<Avatar name="Grace" className="my-custom" />)
    expect(container.firstChild).toHaveClass("my-custom")
  })
})
