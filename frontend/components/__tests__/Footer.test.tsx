import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { NextIntlClientProvider } from "next-intl"

// Stub the next-intl navigation Link to a plain anchor so we can assert href
// without dragging in the full next-intl + next/navigation runtime.
vi.mock("@/i18n/navigation", () => ({
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
}))

import Footer from "@/components/Footer"

const messages = {
  footer: {
    terms: "Terms",
    privacy: "Privacy",
    guidelines: "Guidelines",
    safety: "Safety",
    copyright: "© {year} Circl",
    navAriaLabel: "Legal links",
  },
}

function renderWithIntl(ui: React.ReactElement) {
  return render(
    <NextIntlClientProvider locale="en" messages={messages}>
      {ui}
    </NextIntlClientProvider>,
  )
}

describe("Footer", () => {
  it("renders all four legal links with correct hrefs", () => {
    renderWithIntl(<Footer />)
    expect(screen.getByRole("link", { name: "Terms" })).toHaveAttribute("href", "/en/terms")
    expect(screen.getByRole("link", { name: "Privacy" })).toHaveAttribute("href", "/en/privacy")
    expect(screen.getByRole("link", { name: "Guidelines" })).toHaveAttribute("href", "/en/guidelines")
    expect(screen.getByRole("link", { name: "Safety" })).toHaveAttribute("href", "/en/safety")
  })

  it("interpolates the current year into the copyright string", () => {
    renderWithIntl(<Footer />)
    const year = new Date().getFullYear()
    expect(screen.getByText(`© ${year} Circl`)).toBeInTheDocument()
  })

  it("labels the navigation landmark for screen readers", () => {
    renderWithIntl(<Footer />)
    expect(screen.getByRole("navigation", { name: "Legal links" })).toBeInTheDocument()
  })
})
