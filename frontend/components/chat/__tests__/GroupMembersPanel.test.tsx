import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen, fireEvent, waitFor, within } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"

const API = "http://localhost:8080"

import GroupMembersPanel from "@/components/chat/GroupMembersPanel"

const ADMIN = { user_id: "me", username: "ada", display_name: "Ada", avatar_url: "", is_admin: true, joined_at: "2026-01-01T00:00:00Z" }
const MEMBER = { user_id: "u2", username: "bea", display_name: "Bea", avatar_url: "", is_admin: false, joined_at: "2026-01-02T00:00:00Z" }

const baseProps = {
  roomId: "r1",
  roomName: "Team",
  roomType: "group" as const,
  currentUserId: "me",
  token: "tok",
  onClose: vi.fn(),
  onNameUpdated: vi.fn(),
  onLeft: vi.fn(),
}

function mockMembers(list: unknown[]) {
  server.use(http.get(`${API}/chat/rooms/r1/members`, () => HttpResponse.json(list)))
}

describe("GroupMembersPanel", () => {
  beforeEach(() => {
    baseProps.onClose.mockClear()
    baseProps.onNameUpdated.mockClear()
    baseProps.onLeft.mockClear()
    mockMembers([ADMIN, MEMBER])
  })

  it("loads and lists the members", async () => {
    renderWithIntl(<GroupMembersPanel {...baseProps} />)
    expect(await screen.findByText("Ada")).toBeInTheDocument()
    expect(screen.getByText("Bea")).toBeInTheDocument()
    expect(screen.getByText("Group members")).toBeInTheDocument()
  })

  it("shows the channel title for channel rooms", async () => {
    renderWithIntl(<GroupMembersPanel {...baseProps} roomType="channel" />)
    expect(await screen.findByText("Channel members")).toBeInTheDocument()
  })

  it("surfaces a load error", async () => {
    server.use(http.get(`${API}/chat/rooms/r1/members`, () => new HttpResponse(null, { status: 500 })))
    renderWithIntl(<GroupMembersPanel {...baseProps} />)
    expect(await screen.findByText("Failed to load members.")).toBeInTheDocument()
  })

  it("closes the panel via the back control", async () => {
    renderWithIntl(<GroupMembersPanel {...baseProps} />)
    await screen.findByText("Ada")
    fireEvent.click(screen.getByRole("button", { name: /close/i }))
    expect(baseProps.onClose).toHaveBeenCalled()
  })

  it("renames the group as an admin", async () => {
    server.use(http.put(`${API}/chat/rooms/r1`, () => HttpResponse.json({ ok: true })))
    renderWithIntl(<GroupMembersPanel {...baseProps} />)
    await screen.findByText("Ada")
    fireEvent.click(screen.getByRole("button", { name: /rename group/i }))
    const input = screen.getByLabelText(/group name/i)
    fireEvent.change(input, { target: { value: "Renamed" } })
    fireEvent.click(screen.getByRole("button", { name: /save/i }))
    await waitFor(() => expect(baseProps.onNameUpdated).toHaveBeenCalledWith("Renamed"))
  })

  it("adds a contact as a new member", async () => {
    let posted: { user_id?: string } | null = null
    server.use(
      http.get(`${API}/contacts`, () =>
        HttpResponse.json([{ user_id: "u3", username: "cid", email: "", display_name: "Cid", avatar_url: "" }]),
      ),
      http.post(`${API}/chat/rooms/r1/members`, async ({ request }) => {
        posted = (await request.json()) as { user_id?: string }
        return HttpResponse.json({ ok: true })
      }),
    )
    renderWithIntl(<GroupMembersPanel {...baseProps} />)
    await screen.findByText("Ada")
    fireEvent.click(screen.getByRole("button", { name: /add member/i }))
    const row = (await screen.findByText("Cid")).closest("li") as HTMLElement
    fireEvent.click(within(row).getByRole("button", { name: "Add" }))
    await waitFor(() => expect(posted).toEqual({ user_id: "u3" }))
  })

  it("removes a non-admin member after confirmation", async () => {
    let members = [ADMIN, MEMBER]
    server.use(
      http.get(`${API}/chat/rooms/r1/members`, () => HttpResponse.json(members)),
      http.delete(`${API}/chat/rooms/r1/members/u2`, () => {
        members = [ADMIN]
        return HttpResponse.json({ ok: true })
      }),
    )
    renderWithIntl(<GroupMembersPanel {...baseProps} />)
    await screen.findByText("Bea")
    fireEvent.click(screen.getByRole("button", { name: "Remove" }))
    const dialog = (await screen.findByText("Remove member")).closest('[role="dialog"]') as HTMLElement
    fireEvent.click(within(dialog).getByRole("button", { name: /^remove$/i }))
    await waitFor(() => expect(screen.queryByText("Bea")).not.toBeInTheDocument())
  })

  it("leaves a channel immediately without a delete call", async () => {
    const deleteSpy = vi.fn()
    mockMembers([{ ...MEMBER, user_id: "me", display_name: "Ada", is_admin: false }])
    server.use(http.delete(`${API}/chat/rooms/r1/members/me`, () => { deleteSpy(); return HttpResponse.json({ ok: true }) }))
    renderWithIntl(<GroupMembersPanel {...baseProps} roomType="channel" />)
    await screen.findByText("Ada")
    fireEvent.click(screen.getByRole("button", { name: /leave/i }))
    const dialog = (await screen.findByText("Leave channel")).closest('[role="dialog"]') as HTMLElement
    fireEvent.click(within(dialog).getByRole("button", { name: /^leave$/i }))
    await waitFor(() => expect(baseProps.onLeft).toHaveBeenCalled())
    expect(deleteSpy).not.toHaveBeenCalled()
  })

  it("does not offer rename or remove controls to non-admins", async () => {
    mockMembers([{ ...ADMIN, is_admin: false }, MEMBER])
    renderWithIntl(<GroupMembersPanel {...baseProps} />)
    await screen.findByText("Ada")
    expect(screen.queryByRole("button", { name: /rename group/i })).not.toBeInTheDocument()
    expect(screen.queryByRole("button", { name: "Remove" })).not.toBeInTheDocument()
  })
})
