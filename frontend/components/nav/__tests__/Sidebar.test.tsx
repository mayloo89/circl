import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen, fireEvent, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"

const API = "http://localhost:8080"

let pathname = "/"
let session: Record<string, unknown> | null = { accessToken: "tok", refreshToken: "ref" }
let collapsed = false
let push = { permission: "default" as string, supported: true, enable: vi.fn(), disable: vi.fn() }
const signOut = vi.fn(() => Promise.resolve())
const replace = vi.fn()
const toggleSidebar = vi.fn()

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: session, status: "authenticated" }),
  signOut: () => signOut(),
}))

vi.mock("@/i18n/navigation", () => ({
  usePathname: () => pathname,
  useRouter: () => ({ replace, push: vi.fn(), back: vi.fn() }),
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
}))

vi.mock("@/contexts/NotificationsContext", () => ({
  useNotificationsContext: () => ({ pendingCount: 2, unreadChatCount: 5 }),
}))
vi.mock("@/contexts/ProfileContext", () => ({
  useProfileContext: () => ({ profile: { display_name: "Ada", avatar_url: "" } }),
}))
vi.mock("@/contexts/SidebarContext", () => ({
  useSidebar: () => ({ collapsed, toggle: toggleSidebar }),
}))
vi.mock("@/contexts/PushContext", () => ({ usePushContext: () => push }))
vi.mock("@/contexts/ThemeContext", () => ({ useTheme: () => ({ theme: "dark", toggle: vi.fn() }) }))

import Sidebar from "@/components/nav/Sidebar"

describe("Sidebar", () => {
  beforeEach(() => {
    pathname = "/"
    session = { accessToken: "tok", refreshToken: "ref" }
    collapsed = false
    push = { permission: "default", supported: true, enable: vi.fn(), disable: vi.fn() }
    signOut.mockClear()
    replace.mockClear()
    toggleSidebar.mockClear()
  })

  it("renders the primary navigation with badge counts", () => {
    renderWithIntl(<Sidebar />)
    expect(screen.getByRole("link", { name: /Home/ })).toHaveAttribute("href", "/en/")
    expect(screen.getByRole("link", { name: /Browse/ })).toBeInTheDocument()
    expect(screen.getByRole("link", { name: /Albums/ })).toBeInTheDocument()
    // unread chat (5) and pending (2) badges
    expect(screen.getByText("5")).toBeInTheDocument()
    expect(screen.getByText("2")).toBeInTheDocument()
  })

  it("marks the active route with aria-current", () => {
    pathname = "/contacts"
    renderWithIntl(<Sidebar />)
    expect(screen.getByRole("link", { name: /Contacts/ })).toHaveAttribute("aria-current", "page")
  })

  it("collapses via the toggle button", () => {
    renderWithIntl(<Sidebar />)
    fireEvent.click(screen.getByRole("button", { name: "Collapse sidebar" }))
    expect(toggleSidebar).toHaveBeenCalledOnce()
  })

  it("shows the expand button and hides labels when collapsed", () => {
    collapsed = true
    renderWithIntl(<Sidebar />)
    expect(screen.getByRole("button", { name: "Expand sidebar" })).toBeInTheDocument()
    // language switcher only renders when expanded
    expect(screen.queryByText("Language")).toBeNull()
  })

  it("shows the admin entry only for admins", () => {
    const { unmount } = renderWithIntl(<Sidebar />)
    expect(screen.queryByRole("link", { name: /Admin panel/ })).toBeNull()
    unmount()

    session = { accessToken: "tok", refreshToken: "ref", role: "admin" }
    renderWithIntl(<Sidebar />)
    expect(screen.getByRole("link", { name: /Admin panel/ })).toHaveAttribute("href", "/en/admin")
  })

  it("signs out, revoking presence and refresh token first", async () => {
    let heartbeatDeleted = false
    let logoutCalled = false
    server.use(
      http.delete(`${API}/presence/heartbeat`, () => {
        heartbeatDeleted = true
        return new HttpResponse(null, { status: 204 })
      }),
      http.post(`${API}/auth/logout`, () => {
        logoutCalled = true
        return HttpResponse.json({})
      }),
    )
    renderWithIntl(<Sidebar />)
    fireEvent.click(screen.getByRole("button", { name: "Log out" }))
    await waitFor(() => expect(signOut).toHaveBeenCalledOnce())
    expect(heartbeatDeleted).toBe(true)
    expect(logoutCalled).toBe(true)
  })

  it("persists the chosen locale and navigates", async () => {
    let savedLocale = ""
    server.use(
      http.put(`${API}/profiles/me/preferences`, async ({ request }) => {
        const body = (await request.json()) as { locale: string }
        savedLocale = body.locale
        return HttpResponse.json({})
      }),
    )
    renderWithIntl(<Sidebar />)
    fireEvent.click(screen.getByRole("button", { name: "ES" }))
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/", { locale: "es" }))
    expect(savedLocale).toBe("es")
  })

  it("toggles push from the sidebar control", () => {
    push.permission = "granted"
    renderWithIntl(<Sidebar />)
    fireEvent.click(screen.getByRole("button", { name: "Disable push notifications" }))
    expect(push.disable).toHaveBeenCalledOnce()
  })
})
