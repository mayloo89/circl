import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen, fireEvent, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"
import CreateGroupModal from "@/components/chat/CreateGroupModal"

const API = "http://localhost:8080"

const contacts = [
  { contact_id: "c1", user_id: "u1", username: "ada", email: "ada@x.z", display_name: "Ada", avatar_url: "" },
  { contact_id: "c2", user_id: "u2", username: "bo", email: "bo@x.z", display_name: "Bo", avatar_url: "" },
]

function render(props = {}) {
  const onClose = vi.fn()
  const onCreated = vi.fn()
  renderWithIntl(<CreateGroupModal open token="tok" onClose={onClose} onCreated={onCreated} {...props} />)
  return { onClose, onCreated }
}

describe("CreateGroupModal", () => {
  beforeEach(() => {
    server.use(http.get(`${API}/contacts`, () => HttpResponse.json(contacts)))
  })

  it("loads contacts and disables create until a name is entered", async () => {
    render()
    expect(await screen.findByText("Ada")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: "Create group" })).toBeDisabled()
  })

  it("tracks the selected-member count", async () => {
    render()
    fireEvent.click(await screen.findByText("Ada"))
    expect(screen.getByText("Add contacts (1 selected)")).toBeInTheDocument()
    fireEvent.click(screen.getByText("Ada"))
    expect(screen.getByText("Add contacts")).toBeInTheDocument()
  })

  it("creates a group with the trimmed name and selected members", async () => {
    let payload: { name: string; member_ids: string[] } | null = null
    server.use(
      http.post(`${API}/chat/rooms`, async ({ request }) => {
        payload = (await request.json()) as { name: string; member_ids: string[] }
        return HttpResponse.json({ id: "grp-1" })
      }),
    )
    const { onCreated } = render()
    fireEvent.click(await screen.findByText("Ada"))
    fireEvent.change(screen.getByLabelText("Group name"), { target: { value: "  Crew  " } })
    fireEvent.click(screen.getByRole("button", { name: "Create group" }))
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith("grp-1"))
    expect(payload).toEqual({ name: "Crew", member_ids: ["u1"] })
  })

  it("surfaces a server error on failed creation", async () => {
    server.use(
      http.post(`${API}/chat/rooms`, () =>
        HttpResponse.json({ error: "Limit reached" }, { status: 400 }),
      ),
    )
    render()
    await screen.findByText("Ada")
    fireEvent.change(screen.getByLabelText("Group name"), { target: { value: "Crew" } })
    fireEvent.click(screen.getByRole("button", { name: "Create group" }))
    expect(await screen.findByText("Limit reached")).toBeInTheDocument()
  })

  it("shows a load error when contacts fail to load", async () => {
    server.use(http.get(`${API}/contacts`, () => new HttpResponse(null, { status: 500 })))
    render()
    expect(await screen.findByText("Failed to load contacts.")).toBeInTheDocument()
  })

  it("closes via cancel", async () => {
    const { onClose } = render()
    await screen.findByText("Ada")
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }))
    expect(onClose).toHaveBeenCalledOnce()
  })
})
