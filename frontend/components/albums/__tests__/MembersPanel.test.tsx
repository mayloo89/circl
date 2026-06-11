import { describe, it, expect, beforeEach } from "vitest"
import { screen, fireEvent, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"
import MembersPanel from "@/components/albums/MembersPanel"
import type { Grant } from "@/lib/albums"

const API = "http://localhost:8080"

const contacts = [
  { user_id: "u2", username: "bea", email: "", display_name: "Bea", avatar_url: "" },
  { user_id: "u3", username: "carla", email: "", display_name: "Carla", avatar_url: "" },
]

function grant(overrides: Partial<Grant> = {}): Grant {
  return {
    id: "g1",
    album_id: "a1",
    granter_id: "u1",
    grantee_id: "u2",
    status: "active",
    source: "invite",
    requested_at: "2026-06-01T00:00:00Z",
    expires_at: null,
    ...overrides,
  }
}

function setup(grants: Grant[]) {
  server.use(
    http.get(`${API}/albums/a1/grants`, () => HttpResponse.json(grants)),
    http.get(`${API}/contacts`, () => HttpResponse.json(contacts)),
  )
  return renderWithIntl(<MembersPanel albumID="a1" token="tok" />)
}

describe("MembersPanel", () => {
  beforeEach(() => {
    document.body.style.overflow = ""
  })

  it("shows the empty state when nobody has access", async () => {
    setup([])
    expect(await screen.findByText("No one has access yet.")).toBeInTheDocument()
  })

  it("lists active grants with permanent access by default", async () => {
    setup([grant()])
    expect(await screen.findByText("Bea")).toBeInTheDocument()
    expect(screen.getByText("Active")).toBeInTheDocument()
    expect(screen.getByText("Permanent access")).toBeInTheDocument()
  })

  it("shows remaining time on expiring grants", async () => {
    const inTwoDays = new Date(Date.now() + 2 * 24 * 60 * 60 * 1000 + 60_000).toISOString()
    setup([grant({ expires_at: inTwoDays })])
    expect(await screen.findByText("3 days left")).toBeInTheDocument()
  })

  it("excludes contacts with an open grant from the invite picker", async () => {
    setup([grant({ grantee_id: "u2" })])
    await screen.findByText("Bea")
    const select = screen.getByLabelText("Invite contact") as HTMLSelectElement
    const values = Array.from(select.options).map((o) => o.value)
    expect(values).not.toContain("u2")
    expect(values).toContain("u3")
  })

  it("invites a contact with the chosen expiry preset", async () => {
    let body: unknown
    server.use(
      http.post(`${API}/albums/a1/grants/invite`, async ({ request }) => {
        body = await request.json()
        return HttpResponse.json(grant({ id: "g-new", grantee_id: "u3" }))
      }),
    )
    setup([])
    // The empty state renders before the contacts fetch resolves — wait for
    // the option to exist, or the select change is a no-op in jsdom.
    await screen.findByRole("option", { name: "Carla" })

    fireEvent.change(screen.getByLabelText("Invite contact"), { target: { value: "u3" } })
    fireEvent.click(screen.getByRole("button", { name: "7 days" }))
    fireEvent.click(screen.getByRole("button", { name: "Invite" }))

    // "Carla" also matches the picker <option>, so wait on the request body
    // and the new member row's status instead.
    await waitFor(() => expect(body).toEqual({ grantee_id: "u3", expires_in: "7d" }))
    expect(await screen.findByText("Active")).toBeInTheDocument()
  })

  it("revokes access only after confirmation", async () => {
    let revoked = ""
    server.use(
      http.post(`${API}/albums/grants/:id/revoke`, ({ params }) => {
        revoked = params.id as string
        return HttpResponse.json(grant({ status: "revoked" }))
      }),
    )
    setup([grant()])
    await screen.findByText("Bea")

    fireEvent.click(screen.getByRole("button", { name: "Revoke access" }))
    expect(revoked).toBe("")
    expect(screen.getByText("Revoke this person's access?")).toBeInTheDocument()

    // The dialog's confirm button reuses the "Revoke access" label.
    const dialog = screen.getByRole("dialog")
    fireEvent.click(dialog.querySelector("button:last-of-type")!)
    await waitFor(() => expect(revoked).toBe("g1"))
    // Revoked grants drop off the open-members list (Bea reappears in the
    // invite picker, so assert on the empty members state instead).
    expect(await screen.findByText("No one has access yet.")).toBeInTheDocument()
  })

  it("lets the owner accept a pending access request", async () => {
    let accepted = ""
    server.use(
      http.post(`${API}/albums/grants/:id/accept`, ({ params }) => {
        accepted = params.id as string
        return HttpResponse.json(grant({ status: "active", source: "request" }))
      }),
    )
    setup([grant({ status: "pending", source: "request" })])
    await screen.findByText("Pending — they asked, accept to grant")

    fireEvent.click(screen.getByRole("button", { name: "Accept" }))
    await waitFor(() => expect(accepted).toBe("g1"))
    expect(await screen.findByText("Active")).toBeInTheDocument()
  })

  it("surfaces invite failures", async () => {
    server.use(
      http.post(`${API}/albums/a1/grants/invite`, () =>
        HttpResponse.json({ error: "rate limited" }, { status: 429 }),
      ),
    )
    setup([])
    await screen.findByRole("option", { name: "Carla" })
    fireEvent.change(screen.getByLabelText("Invite contact"), { target: { value: "u3" } })
    fireEvent.click(screen.getByRole("button", { name: "Invite" }))
    expect(await screen.findByRole("alert")).toHaveTextContent("Failed to invite contact.")
  })
})
