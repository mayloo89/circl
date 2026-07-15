import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen, fireEvent, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"

const API = "http://localhost:8080"

const push = vi.fn()

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { accessToken: "tok" }, status: "authenticated" }),
}))

vi.mock("@/i18n/navigation", () => ({
  useRouter: () => ({ push, back: vi.fn(), replace: vi.fn() }),
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
}))

vi.mock("@/contexts/NotificationsContext", () => ({
  useNotificationsContext: () => ({
    pendingCount: 0,
    unreadChatCount: 0,
    clearChatBadge: vi.fn(),
    subscribe: () => () => {},
  }),
}))

import ChatListPane from "@/components/chat/ChatListPane"

const DM = {
  id: "r1",
  type: "dm",
  name: "",
  peer_id: "u2",
  peer_name: "Bea",
  peer_avatar_url: "",
  last_message: { sender_id: "u2", content: "hey there", type: "text", created_at: new Date().toISOString() },
  unread_count: 3,
  created_at: "2026-01-01T00:00:00Z",
}
const GROUP = {
  id: "r2",
  type: "group",
  name: "Team",
  peer_id: "",
  peer_name: "",
  peer_avatar_url: "",
  last_message: null,
  unread_count: 0,
  created_at: "2026-01-01T00:00:00Z",
}

function mockRooms(rooms: unknown[], contacts: unknown[] = [{ user_id: "u2" }]) {
  server.use(
    http.get(`${API}/chat/rooms`, () => HttpResponse.json(rooms)),
    http.get(`${API}/contacts`, () => HttpResponse.json(contacts)),
  )
}

describe("ChatListPane", () => {
  beforeEach(() => {
    push.mockClear()
    mockRooms([DM, GROUP])
  })

  it("renders the room list with peer names and unread badges", async () => {
    renderWithIntl(<ChatListPane />)
    expect(await screen.findByText("Bea")).toBeInTheDocument()
    expect(screen.getByText("Team")).toBeInTheDocument()
    expect(screen.getByText("3")).toBeInTheDocument()
    expect(screen.getByText("hey there")).toBeInTheDocument()
  })

  it("filters rooms by the search query", async () => {
    renderWithIntl(<ChatListPane />)
    await screen.findByText("Bea")
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "team" } })
    expect(screen.queryByText("Bea")).not.toBeInTheDocument()
    expect(screen.getByText("Team")).toBeInTheDocument()
  })

  it("shows the no-results message when nothing matches", async () => {
    renderWithIntl(<ChatListPane />)
    await screen.findByText("Bea")
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "zzz" } })
    expect(screen.getByText("No conversations match your search.")).toBeInTheDocument()
  })

  it("shows the empty state with a contacts link when there are no rooms", async () => {
    mockRooms([])
    renderWithIntl(<ChatListPane />)
    expect(await screen.findByText("No conversations yet")).toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "Go to contacts" }))
    expect(push).toHaveBeenCalledWith("/contacts")
  })

  it("renders an error with a working retry", async () => {
    let fail = true
    server.use(
      http.get(`${API}/chat/rooms`, () => (fail ? new HttpResponse(null, { status: 500 }) : HttpResponse.json([DM]))),
      http.get(`${API}/contacts`, () => HttpResponse.json([{ user_id: "u2" }])),
    )
    renderWithIntl(<ChatListPane />)
    expect(await screen.findByText("Failed to load conversations.")).toBeInTheDocument()
    fail = false
    fireEvent.click(screen.getByRole("button", { name: "Retry" }))
    expect(await screen.findByText("Bea")).toBeInTheDocument()
  })

  it("disables the compose buttons when the user has no contacts", async () => {
    mockRooms([DM], [])
    renderWithIntl(<ChatListPane />)
    await screen.findByText("Bea")
    await waitFor(() => expect(screen.getByRole("button", { name: "New chat" })).toBeDisabled())
    expect(screen.getByRole("button", { name: "New group" })).toBeDisabled()
  })

  it("opens the new-chat modal from the compose button", async () => {
    renderWithIntl(<ChatListPane />)
    await screen.findByText("Bea")
    await waitFor(() => expect(screen.getByRole("button", { name: "New chat" })).toBeEnabled())
    fireEvent.click(screen.getByRole("button", { name: "New chat" }))
    expect(await screen.findByRole("dialog")).toBeInTheDocument()
  })

  it("marks the selected room with aria-current", async () => {
    renderWithIntl(<ChatListPane selectedRoomId="r1" />)
    await screen.findByText("Bea")
    const link = screen.getByText("Bea").closest("a")
    expect(link).toHaveAttribute("aria-current", "page")
  })
})
