import { describe, it, expect, vi } from "vitest"
import { screen, within } from "@testing-library/react"

// Stub next-intl navigation: Link → plain anchor with the /en prefix; the
// hooks are render-only here (router.replace is wired but never fired).
vi.mock("@/i18n/navigation", () => ({
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
  usePathname: () => "/",
  useRouter: () => ({ replace: vi.fn() }),
}))

vi.mock("@/contexts/ThemeContext", () => ({
  useTheme: () => ({ theme: "dark", toggle: vi.fn() }),
}))

import { renderWithIntl } from "@/test/renderWithIntl"
import Landing from "@/components/landing/Landing"

describe("Landing", () => {
  it("renders the hero with both primary conversion CTAs", () => {
    renderWithIntl(<Landing />)

    expect(
      screen.getByRole("heading", { level: 1, name: "Someone nearby is worth meeting." }),
    ).toBeInTheDocument()

    // Every "create account" CTA (header, hero, final band) points at register.
    const registerLinks = screen
      .getAllByRole("link")
      .filter((a) => a.getAttribute("href") === "/en/register")
    expect(registerLinks.length).toBeGreaterThanOrEqual(2)
  })

  it("offers the no-account public-rooms entry and a login path", () => {
    renderWithIntl(<Landing />)

    const roomsLinks = screen
      .getAllByRole("link")
      .filter((a) => a.getAttribute("href") === "/en/rooms")
    expect(roomsLinks.length).toBeGreaterThanOrEqual(1)

    expect(screen.getByRole("link", { name: "Log in" })).toHaveAttribute("href", "/en/login")
  })

  it("lists the four privacy guarantees", () => {
    renderWithIntl(<Landing />)
    expect(screen.getByText("Presence is shown only to people you have accepted.")).toBeInTheDocument()
    expect(screen.getByText("Public profiles hide your birth date, email, and exact location.")).toBeInTheDocument()
    expect(screen.getByText("Adults-only community with active moderation and easy reporting.")).toBeInTheDocument()
    expect(screen.getByText("Leave whenever you want and take your data with you.")).toBeInTheDocument()
  })

  it("exposes a skip link to the main content", () => {
    renderWithIntl(<Landing />)
    expect(screen.getByRole("link", { name: "Skip to main content" })).toHaveAttribute(
      "href",
      "#landing-main",
    )
  })

  it("offers a localized language switcher", () => {
    renderWithIntl(<Landing />)
    const group = screen.getByRole("group", { name: "Language" })
    expect(within(group).getByRole("button", { name: "EN" })).toHaveAttribute("aria-pressed", "true")
    expect(within(group).getByRole("button", { name: "ES" })).toHaveAttribute("aria-pressed", "false")
  })
})
