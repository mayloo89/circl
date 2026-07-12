import { describe, it, expect, vi } from "vitest"
import { screen } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"

vi.mock("@/i18n/navigation", () => ({
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
}))

import LegalPage from "@/components/LegalPage"

describe("LegalPage", () => {
  it("renders the title, updated date, back link and body", () => {
    renderWithIntl(
      <LegalPage title="Privacy Policy" lastUpdated="2026-05-10">
        <p>Body content</p>
      </LegalPage>,
    )
    expect(screen.getByRole("heading", { name: "Privacy Policy" })).toBeInTheDocument()
    expect(screen.getByText("Last updated: 2026-05-10")).toBeInTheDocument()
    expect(screen.getByText("Body content")).toBeInTheDocument()
    expect(screen.getByRole("link", { name: /Back/ })).toHaveAttribute("href", "/en/")
  })

  it("hides the draft banner by default", () => {
    renderWithIntl(
      <LegalPage title="Terms" lastUpdated="2026-01-01">
        <p>Terms body</p>
      </LegalPage>,
    )
    expect(screen.queryByRole("note")).toBeNull()
  })

  it("shows the draft banner when flagged", () => {
    renderWithIntl(
      <LegalPage title="Terms" lastUpdated="2026-01-01" draft>
        <p>Terms body</p>
      </LegalPage>,
    )
    expect(screen.getByRole("note")).toHaveTextContent("Draft — pending legal review before launch.")
  })
})
