import { describe, it, expect, beforeEach, vi } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"
import ReportEvidenceFiles from "@/components/admin/ReportEvidenceFiles"

const API = "http://localhost:8080"

function file(overrides: Record<string, unknown> = {}) {
  return {
    id: "f1",
    report_id: "rep1",
    kind: "message_media",
    content_type: "image/jpeg",
    size_bytes: 2048,
    sha256: "ab12",
    created_at: "2026-11-01T12:00:00Z",
    ...overrides,
  }
}

function seed(files: object[], status = 200) {
  server.use(
    http.get(`${API}/reports/rep1/evidence`, ({ request }) =>
      request.headers.get("Authorization") === "Bearer tok"
        ? HttpResponse.json(files, { status })
        : HttpResponse.json({ code: "unauthorized" }, { status: 401 }),
    ),
  )
}

describe("ReportEvidenceFiles", () => {
  beforeEach(() => {
    URL.createObjectURL = vi.fn().mockReturnValue("blob:evidence")
    URL.revokeObjectURL = vi.fn()
  })

  it("lists files hidden until a moderator reveals one", async () => {
    let fetched = 0
    seed([file(), file({ id: "f2", kind: "profile_photo", sha256: "cd34", size_bytes: 3 * 1024 * 1024 })])
    server.use(
      http.get(`${API}/reports/rep1/evidence/f1`, () => {
        fetched++
        return new HttpResponse("img", { headers: { "Content-Type": "image/jpeg" } })
      }),
    )
    renderWithIntl(<ReportEvidenceFiles reportId="rep1" restricted={false} token="tok" />)

    expect(await screen.findByRole("heading", { name: "Preserved media (2 files)" })).toBeInTheDocument()
    expect(screen.getByText("Message attachment")).toBeInTheDocument()
    expect(screen.getByText("Profile photo")).toBeInTheDocument()
    expect(screen.getByText(/2\.0 KB/)).toBeInTheDocument()
    expect(screen.getByText(/3\.0 MB/)).toBeInTheDocument()
    expect(screen.getByText(/sha256:ab12/)).toBeInTheDocument()
    expect(screen.queryByRole("img")).not.toBeInTheDocument()
    expect(fetched).toBe(0)

    fireEvent.click(screen.getAllByRole("button", { name: "Show image" })[0])
    expect(await screen.findByRole("img", { name: "Message attachment" })).toHaveAttribute("src", "blob:evidence")
    expect(fetched).toBe(1)
    expect(screen.getAllByRole("button", { name: "Show image" })).toHaveLength(1)
  })

  it("never offers to reveal restricted evidence", async () => {
    seed([file()])
    renderWithIntl(<ReportEvidenceFiles reportId="rep1" restricted token="tok" />)
    expect(await screen.findByText(/Preserved for law enforcement/)).toBeInTheDocument()
    expect(screen.getByText(/sha256:ab12/)).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Show image" })).not.toBeInTheDocument()
  })

  it("offers no preview for non-image files", async () => {
    seed([file({ content_type: "application/pdf", size_bytes: 12 })])
    renderWithIntl(<ReportEvidenceFiles reportId="rep1" restricted={false} token="tok" />)
    expect(await screen.findByText(/12 B/)).toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Show image" })).not.toBeInTheDocument()
  })

  it("renders nothing when no media was preserved", async () => {
    seed([])
    const { container } = renderWithIntl(<ReportEvidenceFiles reportId="rep1" restricted={false} token="tok" />)
    await vi.waitFor(() => expect(container.querySelector(".animate-pulse")).toBeNull())
    expect(container).toBeEmptyDOMElement()
  })

  it("reports a load failure", async () => {
    seed([], 500)
    renderWithIntl(<ReportEvidenceFiles reportId="rep1" restricted={false} token="tok" />)
    expect(await screen.findByText("Couldn't load the preserved media.")).toBeInTheDocument()
  })
})
