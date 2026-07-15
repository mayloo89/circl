import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen, fireEvent, waitFor, within } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"
import type { ChatMessage } from "@/hooks/useChat"

const API = "http://localhost:8080"

const push = vi.fn()
const send = vi.fn()
const sendTyping = vi.fn()
let sessionStatus = "authenticated"

const chat = vi.hoisted(() => ({
  value: {
    messages: [] as ChatMessage[],
    deletedIds: new Set<string>(),
    connected: true,
    send: () => {},
    sendAttachment: vi.fn(),
    sendTyping: () => {},
    typingUsers: new Map(),
    readReceipts: new Map(),
    participantEvents: [] as unknown[],
  },
}))

vi.mock("@/hooks/useChat", () => ({ useChat: () => chat.value }))
vi.mock("@/hooks/usePresence", () => ({
  usePresence: () => ({ u2: { online: true, last_seen_at: null } }),
  formatLastSeen: () => "recently",
}))
vi.mock("@/hooks/useUpload", () => ({
  useUpload: () => ({
    upload: vi.fn(),
    uploading: false,
    reviewing: false,
    rejection: null,
    clearRejection: vi.fn(),
    pendingReview: null,
    clearPendingReview: vi.fn(),
  }),
}))
vi.mock("@/contexts/NotificationsContext", () => ({
  useNotificationsContext: () => ({ clearChatBadge: vi.fn(), subscribe: () => () => {} }),
}))
vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { accessToken: "tok", user: { id: "me" } }, status: sessionStatus }),
}))
vi.mock("@/i18n/navigation", () => ({ useRouter: () => ({ push, back: vi.fn(), replace: vi.fn() }) }))

import RoomView from "@/components/chat/RoomView"

function msg(overrides: Partial<ChatMessage> = {}): ChatMessage {
  return {
    type: "text",
    id: "m1",
    room_id: "r1",
    sender_id: "u2",
    sender_name: "Bea",
    sender_avatar_url: "",
    content: "hi",
    view_once: false,
    created_at: "2026-06-10T12:00:00Z",
    ...overrides,
  }
}

const DM = { id: "r1", type: "dm", name: "", peer_id: "u2", peer_name: "Bea", peer_avatar_url: "", are_accepted_contacts: true }
const GROUP = { id: "r1", type: "group", name: "Team", peer_id: "" }
const CHANNEL = { id: "r1", type: "channel", name: "General", peer_id: "" }

function seed(room: object, history: object[] = [], members: object[] = []) {
  server.use(
    http.get(`${API}/chat/rooms`, () => HttpResponse.json([room])),
    http.get(`${API}/chat/rooms/r1`, () => HttpResponse.json(room)),
    http.get(`${API}/chat/rooms/r1/messages`, () => HttpResponse.json(history)),
    http.get(`${API}/chat/rooms/r1/members`, () => HttpResponse.json(members)),
    http.put(`${API}/chat/rooms/r1/read`, () => HttpResponse.json({ ok: true })),
  )
}

describe("RoomView", () => {
  beforeEach(() => {
    push.mockClear()
    send.mockClear()
    sendTyping.mockClear()
    sessionStatus = "authenticated"
    chat.value = {
      messages: [],
      deletedIds: new Set(),
      connected: true,
      send,
      sendAttachment: vi.fn(),
      sendTyping,
      typingUsers: new Map(),
      readReceipts: new Map(),
      participantEvents: [],
    }
  })

  it("shows a loading skeleton before the session resolves", () => {
    sessionStatus = "loading"
    seed(DM)
    const { container } = renderWithIntl(<RoomView roomId="r1" surface="messages" />)
    expect(container.querySelector(".animate-pulse")).toBeTruthy()
  })

  it("renders a DM header with the peer name and online presence", async () => {
    seed(DM, [msg({ id: "h1", content: "history line" })])
    renderWithIntl(<RoomView roomId="r1" surface="messages" />)
    expect(await screen.findByText("Bea")).toBeInTheDocument()
    expect(screen.getByText("Online")).toBeInTheDocument()
    expect(await screen.findByText("history line")).toBeInTheDocument()
  })

  it("appends live socket messages to the history", async () => {
    seed(DM, [msg({ id: "h1", content: "history line" })])
    chat.value.messages = [msg({ id: "l1", content: "live line" })]
    renderWithIntl(<RoomView roomId="r1" surface="messages" />)
    expect(await screen.findByText("history line")).toBeInTheDocument()
    expect(screen.getByText("live line")).toBeInTheDocument()
  })

  it("sends a message through the chat hook", async () => {
    seed(DM)
    renderWithIntl(<RoomView roomId="r1" surface="messages" />)
    await screen.findByText("Bea")
    const input = screen.getByRole("textbox")
    fireEvent.change(input, { target: { value: "hello" } })
    fireEvent.keyDown(input, { key: "Enter" })
    expect(send).toHaveBeenCalled()
    expect(send.mock.calls[0][0]).toBe("hello")
  })

  it("blocks the peer after confirmation and returns to the chat list", async () => {
    let blocked = false
    seed(DM)
    server.use(http.post(`${API}/contacts/u2/block`, () => { blocked = true; return HttpResponse.json({ ok: true }) }))
    renderWithIntl(<RoomView roomId="r1" surface="messages" />)
    await screen.findByText("Bea")
    fireEvent.click(screen.getByRole("button", { name: "Block user" }))
    const dialog = (await screen.findByText(/Block Bea/)).closest('[role="dialog"]') as HTMLElement
    fireEvent.click(within(dialog).getByRole("button", { name: "Block" }))
    await waitFor(() => expect(blocked).toBe(true))
    await waitFor(() => expect(push).toHaveBeenCalledWith("/chat"))
  })

  it("renders a group header with the member count and opens the members panel", async () => {
    seed(GROUP, [], [
      { user_id: "me", username: "ada", display_name: "Ada", avatar_url: "", is_admin: true },
      { user_id: "u2", username: "bea", display_name: "Bea", avatar_url: "", is_admin: false },
    ])
    renderWithIntl(<RoomView roomId="r1" surface="messages" />)
    expect(await screen.findByText("Team")).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText("2 members")).toBeInTheDocument())
    fireEvent.click(screen.getByRole("button", { name: "Group settings" }))
    expect(await screen.findByRole("dialog", { name: "Group settings" })).toBeInTheDocument()
  })

  it("renders a channel with its name prefixed and members sidebar open", async () => {
    seed(CHANNEL, [], [{ user_id: "u2", username: "bea", display_name: "Bea", avatar_url: "", is_admin: false }])
    renderWithIntl(<RoomView roomId="r1" surface="channels" />)
    expect(await screen.findByText(/General/)).toBeInTheDocument()
    // Channels auto-open the members sidebar — the seeded member must show.
    expect(await screen.findByText("Bea")).toBeInTheDocument()
  })

  it("renders a tombstone for a deleted message", async () => {
    seed(DM, [msg({ id: "h1", content: "secret" })])
    chat.value.deletedIds = new Set(["h1"])
    renderWithIntl(<RoomView roomId="r1" surface="messages" />)
    await screen.findByText("Bea")
    // The original content is gone and the localized tombstone stands in its place.
    await waitFor(() => expect(screen.queryByText("secret")).not.toBeInTheDocument())
    expect(screen.getByText("Message expired")).toBeInTheDocument()
  })
})
