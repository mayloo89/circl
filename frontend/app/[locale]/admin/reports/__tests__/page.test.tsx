import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"

const API = "http://localhost:8080"

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { accessToken: "tok", user: { id: "admin" } }, status: "authenticated" }),
}))
vi.mock("@/i18n/navigation", () => ({
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
}))

import AdminReportsPage from "@/app/[locale]/admin/reports/page"

function report(overrides: Record<string, unknown> = {}) {
  return {
    id: "rep1",
    reporter_id: "u1",
    reported_user_id: "u2",
    reported_email: "bea@example.com",
    reported_name: "Bea",
    reported_username: "bea",
    reported_avatar: "",
    reported_account_gone: false,
    reason: "harassment",
    priority: "normal",
    description: "",
    status: "pending",
    created_at: "2026-11-01T12:00:00Z",
    ...overrides,
  }
}

const MESSAGE = { id: "m1", room_id: "r1", type: "text", content: "the reported words", sent_at: "2026-11-01T11:59:00Z" }

function seed(reports: object[]) {
  server.use(http.get(`${API}/reports`, () => HttpResponse.json(reports)))
}

describe("AdminReportsPage evidence", () => {
  it("links live accounts and flags message reports", async () => {
    seed([report({ message: MESSAGE })])
    renderWithIntl(<AdminReportsPage />)
    expect(await screen.findByRole("link", { name: "Bea" })).toHaveAttribute("href", "/en/profile/bea")
    expect(screen.getByText("Message")).toBeInTheDocument()
  })

  it("shows the snapshot identity without a profile link once the account is gone", async () => {
    seed([report({ reported_user_id: null, reported_account_gone: true })])
    renderWithIntl(<AdminReportsPage />)
    expect(await screen.findByText("Account deleted")).toBeInTheDocument()
    expect(screen.queryByRole("link", { name: /Bea/ })).not.toBeInTheDocument()
  })

  it("shows the reported message copy in the review dialog", async () => {
    seed([report({ message: MESSAGE })])
    renderWithIntl(<AdminReportsPage />)
    fireEvent.click(await screen.findByRole("button", { name: "Review" }))
    expect(await screen.findByText("the reported words")).toBeInTheDocument()
    expect(screen.getByText("Reported message")).toBeInTheDocument()
  })

  it("explains a purged copy and media messages", async () => {
    seed([
      report({ id: "a", message: { ...MESSAGE, content: null, purged_at: "2026-11-02T00:00:00Z" } }),
      report({ id: "b", reported_name: "Cam", message: { ...MESSAGE, type: "image", content: "https://cdn/x.jpg" } }),
    ])
    renderWithIntl(<AdminReportsPage />)
    const [first, second] = await screen.findAllByRole("button", { name: "Review" })
    fireEvent.click(first)
    expect(await screen.findByText(/message copy was deleted/i)).toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }))
    fireEvent.click(second)
    expect(await screen.findByText(/Media message \(image\)/)).toBeInTheDocument()
    expect(screen.queryByText("https://cdn/x.jpg")).not.toBeInTheDocument()
  })

  it("hides moderation actions when the account no longer exists", async () => {
    seed([report({ reported_user_id: null, reported_account_gone: true })])
    renderWithIntl(<AdminReportsPage />)
    fireEvent.click(await screen.findByRole("button", { name: "Review" }))
    expect(await screen.findByText(/no moderation action can be applied/i)).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Ban" })).not.toBeInTheDocument()
  })

  it("maps a gone-account conflict to a localized error", async () => {
    seed([report()])
    server.use(
      http.put(`${API}/reports/rep1/status`, () =>
        HttpResponse.json({ code: "reported_account_gone", error: "x" }, { status: 409 }),
      ),
    )
    renderWithIntl(<AdminReportsPage />)
    fireEvent.click(await screen.findByRole("button", { name: "Review" }))
    fireEvent.click(await screen.findByRole("button", { name: "Submit" }))
    await waitFor(() => expect(screen.getByText(/no moderation action can be applied/i)).toBeInTheDocument())
  })
})
