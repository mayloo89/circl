import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"

let pathname = "/"
const enable = vi.fn()
const disable = vi.fn()
let push = { permission: "default" as string, supported: true, enable, disable }
let profile: { avatar_url?: string; display_name?: string } | null = null

vi.mock("@/i18n/navigation", () => ({
  usePathname: () => pathname,
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
}))

vi.mock("@/contexts/PushContext", () => ({ usePushContext: () => push }))
vi.mock("@/contexts/ProfileContext", () => ({ useProfileContext: () => ({ profile }) }))

import TopBar from "@/components/nav/TopBar"

describe("TopBar", () => {
  beforeEach(() => {
    pathname = "/"
    push = { permission: "default", supported: true, enable, disable }
    profile = null
    enable.mockClear()
    disable.mockClear()
  })

  it("shows the route title for the current path", () => {
    pathname = "/browse"
    renderWithIntl(<TopBar />)
    expect(screen.getByText("Browse")).toBeInTheDocument()
  })

  it("falls back to the home title on an unknown path", () => {
    pathname = "/something-else"
    renderWithIntl(<TopBar />)
    expect(screen.getByText("Home")).toBeInTheDocument()
  })

  it("matches a nested route to its section title", () => {
    pathname = "/albums/123"
    renderWithIntl(<TopBar />)
    expect(screen.getByText("Albums")).toBeInTheDocument()
  })

  it("offers to enable push when supported and not yet granted", () => {
    renderWithIntl(<TopBar />)
    const btn = screen.getByRole("button", { name: "Enable push notifications" })
    fireEvent.click(btn)
    expect(enable).toHaveBeenCalledOnce()
  })

  it("offers to disable push once granted", () => {
    push.permission = "granted"
    renderWithIntl(<TopBar />)
    fireEvent.click(screen.getByRole("button", { name: "Disable push notifications" }))
    expect(disable).toHaveBeenCalledOnce()
  })

  it("hides the push toggle when unsupported", () => {
    push.supported = false
    renderWithIntl(<TopBar />)
    expect(screen.queryByRole("button", { name: "Enable push notifications" })).toBeNull()
    expect(screen.queryByRole("button", { name: "Disable push notifications" })).toBeNull()
  })

  it("links the avatar to the profile page", () => {
    profile = { display_name: "Ada", avatar_url: "" }
    renderWithIntl(<TopBar />)
    expect(screen.getByRole("link", { name: "Profile" })).toHaveAttribute("href", "/en/profile")
  })
})
