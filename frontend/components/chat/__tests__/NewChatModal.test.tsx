import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen, fireEvent, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"

vi.mock("@/i18n/navigation", () => ({
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
}))

import NewChatModal from "@/components/chat/NewChatModal"

const API = "http://localhost:8080"

const contacts = [
  { contact_id: "c1", user_id: "u1", username: "ada", email: "ada@x.z", display_name: "Ada", avatar_url: "" },
  { contact_id: "c2", user_id: "u2", username: "bo", email: "bo@x.z", display_name: "Bo", avatar_url: "" },
]

function render(props = {}) {
  const onClose = vi.fn()
  const onCreated = vi.fn()
  renderWithIntl(<NewChatModal open token="tok" onClose={onClose} onCreated={onCreated} {...props} />)
  return { onClose, onCreated }
}

describe("NewChatModal", () => {
  beforeEach(() => {
    server.use(http.get(`${API}/contacts`, () => HttpResponse.json(contacts)))
  })

  it("loads and lists accepted contacts sorted by name", async () => {
    render()
    expect(await screen.findByText("Ada")).toBeInTheDocument()
    expect(screen.getByText("Bo")).toBeInTheDocument()
    expect(screen.getByText("2 contacts")).toBeInTheDocument()
  })

  it("filters contacts by the search query", async () => {
    render()
    await screen.findByText("Ada")
    fireEvent.change(screen.getByLabelText("Search contacts…"), { target: { value: "bo" } })
    expect(screen.queryByText("Ada")).toBeNull()
    expect(screen.getByText("Bo")).toBeInTheDocument()
  })

  it("shows an empty-search message when nothing matches", async () => {
    render()
    await screen.findByText("Ada")
    fireEvent.change(screen.getByLabelText("Search contacts…"), { target: { value: "zzz" } })
    expect(screen.getByText("No contacts match your search.")).toBeInTheDocument()
  })

  it("creates a DM room and reports the new id", async () => {
    server.use(
      http.post(`${API}/chat/rooms/dm`, async ({ request }) => {
        const body = (await request.json()) as { peer_id: string }
        expect(body.peer_id).toBe("u1")
        return HttpResponse.json({ id: "room-1" })
      }),
    )
    const { onCreated } = render()
    fireEvent.click(await screen.findByText("Ada"))
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith("room-1"))
  })

  it("surfaces a server error when DM creation fails", async () => {
    server.use(
      http.post(`${API}/chat/rooms/dm`, () =>
        HttpResponse.json({ error: "Blocked" }, { status: 403 }),
      ),
    )
    render()
    fireEvent.click(await screen.findByText("Ada"))
    expect(await screen.findByText("Blocked")).toBeInTheDocument()
  })

  it("shows a load error when the contacts request fails", async () => {
    server.use(http.get(`${API}/contacts`, () => new HttpResponse(null, { status: 500 })))
    render()
    expect(await screen.findByText("Failed to load contacts.")).toBeInTheDocument()
  })

  it("closes via the cancel button", async () => {
    const { onClose } = render()
    await screen.findByText("Ada")
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }))
    expect(onClose).toHaveBeenCalledOnce()
  })
})
