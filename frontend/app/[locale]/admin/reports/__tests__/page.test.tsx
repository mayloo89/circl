import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent, waitFor, within } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"

const API = "http://localhost:8080"

// One object for every render, like a resolved session: a fresh object per
// call would restart the page's fetch effect after each response.
const session = vi.hoisted(() => ({ data: { accessToken: "tok", user: { id: "admin" } }, status: "authenticated" }))
vi.mock("next-auth/react", () => ({ useSession: () => session }))
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

const REVIEW_TITLE = "Review report"

const MESSAGE = { id: "m1", room_id: "r1", type: "text", content: "the reported words", sent_at: "2026-11-01T11:59:00Z" }

function seed(reports: object[], evidence: object[] = []) {
  server.use(
    http.get(`${API}/reports`, () => HttpResponse.json(reports)),
    http.get(`${API}/reports/:id/evidence`, () => HttpResponse.json(evidence)),
  )
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

  it("lists preserved media in the dialog and locks CSAM evidence", async () => {
    const file = { id: "f1", report_id: "rep1", kind: "profile_photo", content_type: "image/png", size_bytes: 10, sha256: "feed", created_at: "2026-11-01T12:00:00Z" }
    seed([report({ reason: "csam", priority: "critical" })], [file])
    renderWithIntl(<AdminReportsPage />)
    fireEvent.click(await screen.findByRole("button", { name: "Review" }))
    expect(await screen.findByText(/sha256:feed/)).toBeInTheDocument()
    expect(screen.getByText(/Preserved for law enforcement/)).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Show image" })).not.toBeInTheDocument()
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

  it("names the review dialog after its heading", async () => {
    seed([report()])
    renderWithIntl(<AdminReportsPage />)
    fireEvent.click(await screen.findByRole("button", { name: "Review" }))
    expect(await screen.findByRole("dialog", { name: REVIEW_TITLE })).toBeInTheDocument()
  })

  it("opens resolved reports read-only so retained evidence stays visible", async () => {
    seed([report({ status: "reviewed", message: MESSAGE })])
    renderWithIntl(<AdminReportsPage />)
    fireEvent.click(await screen.findByRole("button", { name: "View" }))
    const dialog = await screen.findByRole("dialog", { name: "Report details" })
    expect(within(dialog).getByText("the reported words")).toBeInTheDocument()
    expect(within(dialog).queryByRole("button", { name: "Submit" })).not.toBeInTheDocument()
    expect(within(dialog).queryByRole("button", { name: "Ban" })).not.toBeInTheDocument()
    fireEvent.click(within(dialog).getByRole("button", { name: "Close" }))
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
  })

  it("shows the purge notice on dismissed reports", async () => {
    seed([report({ status: "dismissed", message: { ...MESSAGE, content: null, purged_at: "2026-11-02T00:00:00Z" } })])
    renderWithIntl(<AdminReportsPage />)
    fireEvent.click(await screen.findByRole("button", { name: "View" }))
    expect(await screen.findByText(/message copy was deleted/i)).toBeInTheDocument()
  })

  it("explains an already-moderated account", async () => {
    seed([report()])
    server.use(
      http.put(`${API}/reports/rep1/status`, () =>
        HttpResponse.json({ code: "already_moderated", error: "x" }, { status: 409 }),
      ),
    )
    renderWithIntl(<AdminReportsPage />)
    fireEvent.click(await screen.findByRole("button", { name: "Review" }))
    fireEvent.click(await screen.findByRole("button", { name: "Submit" }))
    expect(await screen.findByText(/already suspended or banned/i)).toBeInTheDocument()
  })
})
