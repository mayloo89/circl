import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"

let pathname = "/"
let role: string | undefined
let pendingCount = 0
let unreadChatCount = 0

vi.mock("@/i18n/navigation", () => ({
  usePathname: () => pathname,
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
}))

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: role ? { role } : {}, status: "authenticated" }),
}))

vi.mock("@/contexts/NotificationsContext", () => ({
  useNotificationsContext: () => ({ pendingCount, unreadChatCount }),
}))

import BottomNav from "@/components/nav/BottomNav"

describe("BottomNav", () => {
  beforeEach(() => {
    pathname = "/"
    role = undefined
    pendingCount = 0
    unreadChatCount = 0
  })

  it("renders the primary navigation items", () => {
    renderWithIntl(<BottomNav />)
    expect(screen.getByRole("link", { name: "Home" })).toHaveAttribute("href", "/en/")
    expect(screen.getByRole("link", { name: "Browse" })).toHaveAttribute("href", "/en/browse")
    expect(screen.getByRole("link", { name: "Messages" })).toHaveAttribute("href", "/en/chat")
    expect(screen.getByRole("link", { name: "Contacts" })).toHaveAttribute("href", "/en/contacts")
  })

  it("marks the active route with aria-current", () => {
    pathname = "/browse"
    renderWithIntl(<BottomNav />)
    expect(screen.getByRole("link", { name: "Browse" })).toHaveAttribute("aria-current", "page")
    expect(screen.getByRole("link", { name: "Home" })).not.toHaveAttribute("aria-current")
  })

  it("keeps Messages active on a DM thread but not on channels", () => {
    pathname = "/chat/abc"
    const { unmount } = renderWithIntl(<BottomNav />)
    expect(screen.getByRole("link", { name: "Messages" })).toHaveAttribute("aria-current", "page")
    unmount()

    pathname = "/chat/channels"
    renderWithIntl(<BottomNav />)
    expect(screen.getByRole("link", { name: "Messages" })).not.toHaveAttribute("aria-current")
  })

  it("shows unread and pending badges only when counts are positive", () => {
    unreadChatCount = 3
    pendingCount = 0
    renderWithIntl(<BottomNav />)
    expect(screen.getByText("3")).toBeInTheDocument()
  })

  it("opens the More sheet and lists secondary destinations", () => {
    renderWithIntl(<BottomNav />)
    fireEvent.click(screen.getByRole("button", { name: "More" }))
    expect(screen.getByRole("link", { name: "Profile" })).toHaveAttribute("href", "/en/profile")
    expect(screen.getByRole("link", { name: "Settings" })).toHaveAttribute("href", "/en/settings")
  })

  it("hides the admin entry for regular users and shows it for admins", () => {
    const { unmount } = renderWithIntl(<BottomNav />)
    fireEvent.click(screen.getByRole("button", { name: "More" }))
    expect(screen.queryByRole("link", { name: "Admin panel" })).toBeNull()
    unmount()

    role = "super_admin"
    renderWithIntl(<BottomNav />)
    fireEvent.click(screen.getByRole("button", { name: "More" }))
    expect(screen.getByRole("link", { name: "Admin panel" })).toHaveAttribute("href", "/en/admin")
  })
})
