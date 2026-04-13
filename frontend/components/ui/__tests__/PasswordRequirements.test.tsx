import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import { NextIntlClientProvider } from "next-intl"
import PasswordRequirements, { PASSWORD_RULES } from "@/components/ui/PasswordRequirements"

const messages = {
  passwordRequirements: {
    minLength: "At least 8 characters",
    uppercase: "One uppercase letter",
    lowercase: "One lowercase letter",
    number: "One number",
  },
}

function renderWithIntl(ui: React.ReactElement) {
  return render(
    <NextIntlClientProvider locale="en" messages={messages}>
      {ui}
    </NextIntlClientProvider>,
  )
}

describe("PasswordRequirements", () => {
  it("renders all rules when password is empty", () => {
    renderWithIntl(<PasswordRequirements password="" />)
    expect(screen.getByRole("list")).toBeInTheDocument()
    expect(screen.getAllByRole("listitem")).toHaveLength(PASSWORD_RULES.length)
  })

  it("shows all 4 rules when password is non-empty", () => {
    renderWithIntl(<PasswordRequirements password="a" />)
    expect(screen.getByRole("list")).toBeInTheDocument()
    expect(screen.getAllByRole("listitem")).toHaveLength(PASSWORD_RULES.length)
  })

  it("marks met rules with green text", () => {
    renderWithIntl(<PasswordRequirements password="Abcdefg1" />)
    const items = screen.getAllByRole("listitem")
    items.forEach((item) => {
      expect(item).toHaveClass("text-green-400")
    })
  })

  it("marks unmet rules with gray text", () => {
    renderWithIntl(<PasswordRequirements password="abc" />)
    const items = screen.getAllByRole("listitem")
    // "At least 8 chars" unmet, "uppercase" unmet, "digit" unmet — only lowercase is met
    const gray = items.filter((i) => i.classList.contains("text-gray-500"))
    expect(gray.length).toBeGreaterThan(0)
  })

  it("has accessible list label", () => {
    renderWithIntl(<PasswordRequirements password="Test1" />)
    expect(screen.getByRole("list", { name: "Password requirements" })).toBeInTheDocument()
  })

  it("marks length rule met when password >= 8 chars", () => {
    renderWithIntl(<PasswordRequirements password="abcdefgh" />)
    const lengthItem = screen.getByText("At least 8 characters").closest("li")
    expect(lengthItem).toHaveClass("text-green-400")
  })

  it("marks uppercase rule unmet when no uppercase present", () => {
    renderWithIntl(<PasswordRequirements password="abcdefgh1" />)
    const upperItem = screen.getByText("One uppercase letter").closest("li")
    expect(upperItem).toHaveClass("text-gray-500")
  })

  it("marks uppercase rule met when uppercase present", () => {
    renderWithIntl(<PasswordRequirements password="Abcdefgh1" />)
    const upperItem = screen.getByText("One uppercase letter").closest("li")
    expect(upperItem).toHaveClass("text-green-400")
  })

  it("marks number rule met when digit present", () => {
    renderWithIntl(<PasswordRequirements password="Abcdefg1" />)
    const numItem = screen.getByText("One number").closest("li")
    expect(numItem).toHaveClass("text-green-400")
  })
})
