import { describe, it, expect } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import PasswordField from "@/components/ui/PasswordField"

describe("PasswordField", () => {
  it("renders a password input bound to its label", () => {
    render(<PasswordField id="pw" label="Password" />)
    const input = screen.getByLabelText("Password")
    expect(input).toHaveAttribute("type", "password")
  })

  it("toggles visibility via the eye button", () => {
    render(<PasswordField id="pw" label="Password" />)
    const input = screen.getByLabelText("Password")

    fireEvent.click(screen.getByRole("button", { name: "Show password" }))
    expect(input).toHaveAttribute("type", "text")
    expect(screen.getByRole("button", { name: "Hide password" })).toBeInTheDocument()

    fireEvent.click(screen.getByRole("button", { name: "Hide password" }))
    expect(input).toHaveAttribute("type", "password")
  })

  it("keeps the toggle button out of the tab order", () => {
    render(<PasswordField id="pw" label="Password" />)
    expect(screen.getByRole("button", { name: "Show password" })).toHaveAttribute("tabindex", "-1")
  })

  it("shows the error and suppresses the helper when both are set", () => {
    render(<PasswordField id="pw" label="Password" error="Too short" helper="8+ characters" />)
    expect(screen.getByText("Too short")).toBeInTheDocument()
    expect(screen.queryByText("8+ characters")).not.toBeInTheDocument()
  })

  it("shows the helper when there is no error", () => {
    render(<PasswordField id="pw" label="Password" helper="8+ characters" />)
    expect(screen.getByText("8+ characters")).toBeInTheDocument()
  })

  it("hides the label visually when labelHidden is set", () => {
    render(<PasswordField id="pw" label="Password" labelHidden />)
    expect(screen.getByText("Password")).toHaveClass("sr-only")
  })
})
