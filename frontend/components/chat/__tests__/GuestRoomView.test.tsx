import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen, fireEvent, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"
import type { AnyMessage } from "@/types/chat"

const API = "http://localhost:8080"

const push = vi.fn()
const send = vi.fn()
const sendTyping = vi.fn()

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

vi.mock("@/hooks/useGuestChat", () => ({ useGuestChat: () => chat.value }))
vi.mock("@/i18n/navigation", () => ({ useRouter: () => ({ push, back: vi.fn(), replace: vi.fn() }) }))
vi.mock("@/components/ui/Toast", () => ({ useToast: () => ({ toast: vi.fn() }) }))

import GuestRoomView from "@/components/chat/GuestRoomView"

function msg(overrides: Partial<AnyMessage> = {}): AnyMessage {
  return {
    id: "m1",
    room_id: "r1",
    sender_id: "guest-1",
    sender_name: "Guest",
    sender_avatar_url: "",
    type: "text",
    content: "hi",
    view_once: false,
    created_at: "2026-06-10T12:00:00Z",
    ...overrides,
  }
}

describe("GuestRoomView", () => {
  beforeEach(() => {
    push.mockClear()
    send.mockClear()
    sendTyping.mockClear()
    chat.value = {
      messages: [],
      deletedIds: new Set(),
      connected: true,
      send,
      sendTyping,
      participantEvents: [],
      isKicked: false,
      isMuted: false,
    }
    server.use(
      http.get(`${API}/guest/rooms`, () => HttpResponse.json([{ id: "r1", name: "Public Lobby" }])),
      http.get(`${API}/chat/rooms/r1/messages`, () => HttpResponse.json([msg({ id: "h1", content: "old history" })])),
      http.get(`${API}/guest/rooms/r1/participants`, () => HttpResponse.json([])),
    )
  })

  it("loads the room name and history", async () => {
    renderWithIntl(<GuestRoomView roomId="r1" sessionId="guest-1" />)
    expect(await screen.findByText("Public Lobby")).toBeInTheDocument()
    expect(await screen.findByText("old history")).toBeInTheDocument()
  })

  it("shows the guest badge in the header", async () => {
    renderWithIntl(<GuestRoomView roomId="r1" sessionId="guest-1" />)
    await screen.findByText("Public Lobby")
    expect(screen.getAllByText(/guest/i).length).toBeGreaterThan(0)
  })

  it("merges live messages and deduplicates by id", async () => {
    chat.value.messages = [msg({ id: "h1", content: "old history" }), msg({ id: "l1", content: "live one" })]
    renderWithIntl(<GuestRoomView roomId="r1" sessionId="guest-1" />)
    await waitFor(() => expect(screen.getByText("live one")).toBeInTheDocument())
    // "old history" (id h1) exists in both history and live — must render once
    expect(screen.getAllByText("old history")).toHaveLength(1)
  })

  it("forwards composed messages to the guest chat hook", async () => {
    renderWithIntl(<GuestRoomView roomId="r1" sessionId="guest-1" />)
    await screen.findByText("Public Lobby")
    const input = screen.getByRole("textbox")
    fireEvent.change(input, { target: { value: "hello" } })
    fireEvent.keyDown(input, { key: "Enter" })
    expect(send).toHaveBeenCalledWith("hello")
  })

  it("confirms before leaving because a guest loses their session", async () => {
    renderWithIntl(<GuestRoomView roomId="r1" sessionId="guest-1" />)
    await screen.findByText("Public Lobby")
    fireEvent.click(screen.getByRole("button", { name: /back to rooms/i }))
    // confirmOnLeave → does not navigate until confirmed
    expect(push).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole("button", { name: /leave/i }))
    expect(push).toHaveBeenCalledWith("/rooms")
  })
})
