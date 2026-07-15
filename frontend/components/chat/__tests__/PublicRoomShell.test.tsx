import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen, fireEvent, waitFor, within } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"
import type { AnyMessage } from "@/types/chat"
import type { ParticipantEvent } from "@/hooks/useChat"

const API = "http://localhost:8080"

const toast = vi.fn()
vi.mock("@/components/ui/Toast", () => ({
  useToast: () => ({ toast }),
}))

vi.mock("@/i18n/navigation", () => ({
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
}))

import PublicRoomShell from "@/components/chat/PublicRoomShell"

function message(overrides: Partial<AnyMessage> = {}): AnyMessage {
  return {
    id: "m1",
    room_id: "r1",
    sender_id: "u2",
    sender_name: "Bea",
    sender_avatar_url: "",
    type: "text",
    content: "hello room",
    view_once: false,
    created_at: "2026-06-10T12:00:00Z",
    ...overrides,
  }
}

const baseProps = {
  roomId: "r1",
  roomName: "Lobby",
  messages: [] as AnyMessage[],
  historyLoaded: true,
  connected: true,
  deletedIds: new Set<string>(),
  participantEvents: [] as ParticipantEvent[],
  isOwn: (id: string) => id === "me",
  onBack: vi.fn(),
  onSend: vi.fn(),
  onTyping: vi.fn(),
}

describe("PublicRoomShell", () => {
  beforeEach(() => {
    toast.mockClear()
    baseProps.onBack.mockClear()
    server.use(
      http.get(`${API}/guest/rooms/r1/participants`, () =>
        HttpResponse.json([
          { user_id: "u2", display_name: "Bea", avatar_url: "", is_guest: true },
          { user_id: "u3", display_name: "Cid", avatar_url: "" },
        ]),
      ),
      http.get(`${API}/chat/rooms/r1/members`, () =>
        HttpResponse.json([{ user_id: "u9", display_name: "Registered", avatar_url: "" }]),
      ),
      http.post(`${API}/chat/rooms/r1/mod/kick`, () => HttpResponse.json({ ok: true })),
      http.post(`${API}/chat/rooms/r1/mod/mute`, () => HttpResponse.json({ ok: true })),
    )
  })

  it("renders the room name and messages", () => {
    renderWithIntl(<PublicRoomShell {...baseProps} messages={[message()]} />)
    expect(screen.getByText("Lobby")).toBeInTheDocument()
    expect(screen.getByText("hello room")).toBeInTheDocument()
  })

  it("shows a loading state until history is loaded", () => {
    renderWithIntl(<PublicRoomShell {...baseProps} historyLoaded={false} />)
    expect(screen.queryByRole("log")).not.toBeInTheDocument()
  })

  it("seeds the roster from the guest participants endpoint and opens the rail", async () => {
    renderWithIntl(<PublicRoomShell {...baseProps} />)
    // header count reflects the seeded roster
    await waitFor(() => expect(screen.getByText("2")).toBeInTheDocument())
    fireEvent.click(screen.getByRole("button", { name: "Show members" }))
    await waitFor(() => expect(screen.getAllByText("Bea").length).toBeGreaterThan(0))
    expect(screen.getAllByText("Cid").length).toBeGreaterThan(0)
  })

  it("seeds the roster from the authenticated members endpoint when a token is given", async () => {
    renderWithIntl(<PublicRoomShell {...baseProps} token="tok" />)
    await waitFor(() => expect(screen.getByText("1")).toBeInTheDocument())
  })

  it("applies join and leave participant events on top of the seed", async () => {
    renderWithIntl(
      <PublicRoomShell
        {...baseProps}
        participantEvents={[
          { type: "join", userId: "u4", username: "d", displayName: "Dot", avatarURL: "", isGuest: false },
          { type: "leave", userId: "u3", username: "", displayName: "", avatarURL: "", isGuest: false },
        ]}
      />,
    )
    // seed (u2, u3) with u4 joined and u3 left → u2, u4
    fireEvent.click(screen.getByRole("button", { name: "Show members" }))
    await waitFor(() => expect(screen.getAllByText("Dot").length).toBeGreaterThan(0))
    expect(screen.queryAllByText("Cid")).toHaveLength(0)
  })

  it("sends a message through onSend", () => {
    renderWithIntl(<PublicRoomShell {...baseProps} />)
    const input = screen.getByRole("textbox")
    fireEvent.change(input, { target: { value: "hi there" } })
    fireEvent.keyDown(input, { key: "Enter" })
    expect(baseProps.onSend).toHaveBeenCalledWith("hi there")
  })

  it("calls onBack immediately when confirmOnLeave is false", () => {
    renderWithIntl(<PublicRoomShell {...baseProps} />)
    fireEvent.click(screen.getByRole("button", { name: /back to rooms/i }))
    expect(baseProps.onBack).toHaveBeenCalled()
  })

  it("asks for confirmation before leaving when confirmOnLeave is set", () => {
    renderWithIntl(<PublicRoomShell {...baseProps} confirmOnLeave />)
    fireEvent.click(screen.getByRole("button", { name: /back to rooms/i }))
    expect(baseProps.onBack).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole("button", { name: /leave/i }))
    expect(baseProps.onBack).toHaveBeenCalled()
  })

  it("shows the kicked overlay and returns on acknowledge", () => {
    renderWithIntl(<PublicRoomShell {...baseProps} isKicked />)
    expect(screen.getByText("Removed from room")).toBeInTheDocument()
    // The overlay's acknowledge button is the first "back to rooms" control.
    fireEvent.click(screen.getAllByRole("button", { name: /back to rooms/i })[0])
    expect(baseProps.onBack).toHaveBeenCalled()
  })

  it("disables the composer and shows a notice when muted", () => {
    renderWithIntl(<PublicRoomShell {...baseProps} isMuted />)
    expect(screen.getByRole("status")).toBeInTheDocument()
    // The composer is wired connected={connected && !isMuted}, so muting
    // disables the send control even with the socket connected.
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "blocked" } })
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled()
  })

  it("lets an admin kick a roster member", async () => {
    renderWithIntl(<PublicRoomShell {...baseProps} token="tok" isAdmin viewerId="me" />)
    // members endpoint returns u9 (not the viewer) so a mod trigger appears
    fireEvent.click(screen.getByRole("button", { name: "Show members" }))
    const modBtn = await screen.findAllByRole("button", { name: "Moderation options" })
    fireEvent.click(modBtn[0])
    const menu = await screen.findByRole("menu")
    fireEvent.click(within(menu).getByRole("menuitem", { name: "Kick" }))
    await waitFor(() => expect(toast).toHaveBeenCalled())
  })

  it("filters the roster by the member query", async () => {
    renderWithIntl(<PublicRoomShell {...baseProps} />)
    await waitFor(() => expect(screen.getByText("2")).toBeInTheDocument())
    fireEvent.click(screen.getByRole("button", { name: "Show members" }))
    const filter = await screen.findAllByRole("searchbox")
    fireEvent.change(filter[0], { target: { value: "be" } })
    expect(screen.getAllByText("Bea").length).toBeGreaterThan(0)
    expect(screen.queryAllByText("Cid")).toHaveLength(0)
  })
})
