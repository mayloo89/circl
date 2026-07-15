import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen, fireEvent, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"
import type { AnyMessage } from "@/types/chat"

const API = "http://localhost:8080"

const push = vi.fn()
const send = vi.fn()

let session: Record<string, unknown> = { role: "user" }

const chat = vi.hoisted(() => ({
  value: {
    messages: [] as AnyMessage[],
    deletedIds: new Set<string>(),
    connected: true,
    send: () => {},
    sendTyping: () => {},
    participantEvents: [],
    isKicked: false,
    isMuted: false,
  },
}))

vi.mock("@/hooks/useChat", () => ({ useChat: () => chat.value }))
vi.mock("next-auth/react", () => ({ useSession: () => ({ data: session, status: "authenticated" }) }))
vi.mock("@/i18n/navigation", () => ({ useRouter: () => ({ push, back: vi.fn(), replace: vi.fn() }) }))
vi.mock("@/components/ui/Toast", () => ({ useToast: () => ({ toast: vi.fn() }) }))

import RegisteredPublicRoomView from "@/components/chat/RegisteredPublicRoomView"

function msg(overrides: Partial<AnyMessage> = {}): AnyMessage {
  return {
    id: "m1",
    room_id: "r1",
    sender_id: "u2",
    sender_name: "Bea",
    sender_avatar_url: "",
    type: "text",
    content: "hi",
    view_once: false,
    created_at: "2026-06-10T12:00:00Z",
    ...overrides,
  }
}

describe("RegisteredPublicRoomView", () => {
  beforeEach(() => {
    push.mockClear()
    send.mockClear()
    session = { role: "user" }
    chat.value = {
      messages: [],
      deletedIds: new Set(),
      connected: true,
      send,
      sendTyping: () => {},
      participantEvents: [],
      isKicked: false,
      isMuted: false,
    }
    server.use(
      http.get(`${API}/chat/rooms/r1`, () => HttpResponse.json({ name: "Announcements" })),
      http.get(`${API}/chat/rooms/r1/messages`, () => HttpResponse.json([msg({ id: "h1", content: "old post" })])),
      http.get(`${API}/chat/rooms/r1/members`, () => HttpResponse.json([])),
    )
  })

  it("loads the channel name and history", async () => {
    renderWithIntl(<RegisteredPublicRoomView roomId="r1" token="tok" userID="me" />)
    expect(await screen.findByText("Announcements")).toBeInTheDocument()
    expect(await screen.findByText("old post")).toBeInTheDocument()
  })

  it("merges live messages and deduplicates by id", async () => {
    chat.value.messages = [msg({ id: "h1", content: "old post" }), msg({ id: "l1", content: "fresh post" })]
    renderWithIntl(<RegisteredPublicRoomView roomId="r1" token="tok" userID="me" />)
    await waitFor(() => expect(screen.getByText("fresh post")).toBeInTheDocument())
    expect(screen.getAllByText("old post")).toHaveLength(1)
  })

  it("forwards composed messages to the chat hook", async () => {
    renderWithIntl(<RegisteredPublicRoomView roomId="r1" token="tok" userID="me" />)
    await screen.findByText("Announcements")
    const input = screen.getByRole("textbox")
    fireEvent.change(input, { target: { value: "post this" } })
    fireEvent.keyDown(input, { key: "Enter" })
    expect(send).toHaveBeenCalledWith("post this")
  })

  it("navigates back to the channels list without a leave prompt", async () => {
    renderWithIntl(<RegisteredPublicRoomView roomId="r1" token="tok" userID="me" />)
    await screen.findByText("Announcements")
    fireEvent.click(screen.getByRole("button", { name: /back to rooms/i }))
    expect(push).toHaveBeenCalledWith("/chat/channels")
  })

  it("exposes moderation controls to an admin viewer", async () => {
    session = { role: "admin" }
    server.use(
      http.get(`${API}/chat/rooms/r1/members`, () =>
        HttpResponse.json([{ user_id: "u2", display_name: "Bea", avatar_url: "" }]),
      ),
    )
    renderWithIntl(<RegisteredPublicRoomView roomId="r1" token="tok" userID="me" />)
    await screen.findByText("Announcements")
    fireEvent.click(screen.getByRole("button", { name: "Show members" }))
    expect(await screen.findAllByRole("button", { name: "Moderation options" })).not.toHaveLength(0)
  })
})
