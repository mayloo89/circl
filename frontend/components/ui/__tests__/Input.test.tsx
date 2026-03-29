import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import Input from "@/components/ui/Input"

describe("Input", () => {
  it("renders an input element", () => {
    render(<Input />)
    expect(screen.getByRole("textbox")).toBeInTheDocument()
  })

  it("renders a visible label when label is provided", () => {
    render(<Input label="Email" id="email" />)
    expect(screen.getByLabelText("Email")).toBeInTheDocument()
    expect(screen.getByText("Email")).not.toHaveClass("sr-only")
  })

  it("renders a visually hidden label when labelHidden is true", () => {
    render(<Input label="Search" id="search" labelHidden />)
    expect(screen.getByText("Search")).toHaveClass("sr-only")
  })

  it("renders an error message", () => {
    render(<Input error="This field is required" />)
    expect(screen.getByText("This field is required")).toBeInTheDocument()
  })

  it("renders a helper text when no error is present", () => {
    render(<Input helper="Max 280 characters" />)
    expect(screen.getByText("Max 280 characters")).toBeInTheDocument()
  })

  it("does not render helper when an error is present", () => {
    render(<Input error="Required" helper="Max 280 characters" />)
    expect(screen.getByText("Required")).toBeInTheDocument()
    expect(screen.queryByText("Max 280 characters")).not.toBeInTheDocument()
  })

  it("applies orange border when dirty is true", () => {
    render(<Input dirty />)
    expect(screen.getByRole("textbox")).toHaveClass("border-orange-500")
  })

  it("applies indigo border by default (not dirty)", () => {
    render(<Input />)
    expect(screen.getByRole("textbox")).toHaveClass("border-gray-700")
  })

  it("passes through standard input attributes", () => {
    render(<Input type="password" placeholder="Enter password" />)
    const input = screen.getByPlaceholderText("Enter password")
    expect(input).toHaveAttribute("type", "password")
  })
})
