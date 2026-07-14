import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"

let session: { user?: { name?: string } } | null = null

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: session, status: session ? "authenticated" : "loading" }),
}))
vi.mock("@/contexts/ThemeContext", () => ({ useTheme: () => ({ theme: "dark", toggle: vi.fn() }) }))

import HomeHero from "@/components/home/HomeHero"

describe("HomeHero", () => {
  beforeEach(() => {
    session = null
  })

  it("renders the tagline and no greeting before the session loads", () => {
    renderWithIntl(<HomeHero />)
    expect(screen.getByText("Your private circle")).toBeInTheDocument()
    expect(screen.queryByText(/Hello,/)).toBeNull()
  })

  it("greets the user by first name once authenticated", () => {
    session = { user: { name: "Ada Lovelace" } }
    renderWithIntl(<HomeHero />)
    expect(screen.getByText("Hello, Ada")).toBeInTheDocument()
  })
})
