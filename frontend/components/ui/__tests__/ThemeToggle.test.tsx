import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"

const toggle = vi.fn()
let theme = "dark"

vi.mock("@/contexts/ThemeContext", () => ({
  useTheme: () => ({ theme, toggle }),
}))

import ThemeToggle from "@/components/ui/ThemeToggle"

describe("ThemeToggle", () => {
  beforeEach(() => {
    toggle.mockClear()
  })

  it("offers light mode while dark and calls toggle on click", () => {
    theme = "dark"
    renderWithIntl(<ThemeToggle />)
    const button = screen.getByRole("button", { name: "Switch to light mode" })
    fireEvent.click(button)
    expect(toggle).toHaveBeenCalledOnce()
  })

  it("offers dark mode while light", () => {
    theme = "light"
    renderWithIntl(<ThemeToggle />)
    expect(screen.getByRole("button", { name: "Switch to dark mode" })).toBeInTheDocument()
  })

  it("renders the label text when showLabel is set", () => {
    theme = "dark"
    renderWithIntl(<ThemeToggle showLabel />)
    expect(screen.getByText("Switch to light mode")).toBeInTheDocument()
  })
})
