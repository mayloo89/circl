import { describe, it, expect, vi, beforeEach } from "vitest"
import { act, screen, fireEvent, waitFor, within } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"
import type { ContactEvent } from "@/hooks/useNotifications"

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { accessToken: "tok" }, status: "authenticated" }),
}))

let notify: (e: ContactEvent) => void = () => {}
vi.mock("@/contexts/NotificationsContext", () => ({
  useNotificationsContext: () => ({
    subscribe: (cb: (e: ContactEvent) => void) => {
      notify = cb
      return () => {}
    },
  }),
}))

import PendingRequestsWidget from "@/components/home/PendingRequestsWidget"

const API = "http://localhost:8080"

const pending = [
  { contact_id: "c1", user_id: "u2", display_name: "Bea", avatar_url: "" },
  { contact_id: "c2", user_id: "u3", display_name: "Carla", avatar_url: "" },
]

describe("PendingRequestsWidget", () => {
  beforeEach(() => {
    server.use(http.get(`${API}/contacts/pending`, () => HttpResponse.json(pending)))
  })

  it("renders nothing when there are no pending requests", async () => {
    server.use(http.get(`${API}/contacts/pending`, () => HttpResponse.json([])))
    const { container } = renderWithIntl(<PendingRequestsWidget />)
    await waitFor(() => expect(container).toBeEmptyDOMElement())
  })

  it("lists pending requests", async () => {
    renderWithIntl(<PendingRequestsWidget />)
    expect(await screen.findByText("Bea")).toBeInTheDocument()
    expect(screen.getByText("Carla")).toBeInTheDocument()
  })

  it("accepts a request and removes the row", async () => {
    let accepted = ""
    server.use(
      http.put(`${API}/contacts/:id/accept`, ({ params }) => {
        accepted = params.id as string
        return HttpResponse.json({})
      }),
    )
    renderWithIntl(<PendingRequestsWidget />)
    await screen.findByText("Bea")
    fireEvent.click(screen.getAllByRole("button", { name: "Accept" })[0])
    await waitFor(() => expect(screen.queryByText("Bea")).not.toBeInTheDocument())
    expect(accepted).toBe("c1")
    expect(screen.getByText("Carla")).toBeInTheDocument()
  })

  it("requires inline confirmation before declining", async () => {
    let deleted = ""
    server.use(
      http.delete(`${API}/contacts/:id`, ({ params }) => {
        deleted = params.id as string
        return HttpResponse.json({})
      }),
    )
    renderWithIntl(<PendingRequestsWidget />)
    await screen.findByText("Bea")

    fireEvent.click(screen.getAllByRole("button", { name: "Decline" })[0])
    // Nothing is deleted yet — the row now shows the confirm affordance.
    expect(deleted).toBe("")
    const confirmRow = screen.getByText("Confirm?").closest("li")!

    fireEvent.click(within(confirmRow).getByRole("button", { name: "Decline" }))
    await waitFor(() => expect(screen.queryByText("Bea")).not.toBeInTheDocument())
    expect(deleted).toBe("c1")
  })

  it("cancelling the confirmation keeps the request", async () => {
    renderWithIntl(<PendingRequestsWidget />)
    await screen.findByText("Bea")
    fireEvent.click(screen.getAllByRole("button", { name: "Decline" })[0])
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }))
    expect(screen.queryByText("Confirm?")).not.toBeInTheDocument()
    expect(screen.getByText("Bea")).toBeInTheDocument()
  })

  it("shows an error state with retry when the fetch fails", async () => {
    server.use(http.get(`${API}/contacts/pending`, () => HttpResponse.json({}, { status: 500 })))
    renderWithIntl(<PendingRequestsWidget />)
    expect(await screen.findByText("Couldn’t load")).toBeInTheDocument()

    server.use(http.get(`${API}/contacts/pending`, () => HttpResponse.json(pending)))
    fireEvent.click(screen.getByRole("button", { name: "Retry" }))
    expect(await screen.findByText("Bea")).toBeInTheDocument()
  })

  it("refetches when a contact_request notification arrives", async () => {
    renderWithIntl(<PendingRequestsWidget />)
    await screen.findByText("Bea")

    server.use(
      http.get(`${API}/contacts/pending`, () =>
        HttpResponse.json([
          ...pending,
          { contact_id: "c3", user_id: "u4", display_name: "Diego", avatar_url: "" },
        ]),
      ),
    )
    act(() => notify({ type: "contact_request", payload: { contact_id: "c3", requester_id: "u4" } }))
    expect(await screen.findByText("Diego")).toBeInTheDocument()
  })

  it("drops a request when its contact is removed elsewhere", async () => {
    renderWithIntl(<PendingRequestsWidget />)
    await screen.findByText("Bea")
    act(() => notify({ type: "contact_removed", payload: { contact_id: "c1" } }))
    await waitFor(() => expect(screen.queryByText("Bea")).not.toBeInTheDocument())
  })
})
