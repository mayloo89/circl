import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"
import ProfileHeader, { type ContactStatus } from "@/components/profile/ProfileHeader"

const profile = {
  user_id: "u1",
  display_name: "Ada",
  bio: "Loves math",
  avatar_url: "",
  photos: [],
}

function setup(status: ContactStatus, extra = {}) {
  const handlers = {
    onAddContact: vi.fn(),
    onAccept: vi.fn(),
    onStartDM: vi.fn(),
    onBlock: vi.fn(),
    onUnblock: vi.fn(),
    onReport: vi.fn(),
  }
  renderWithIntl(
    <ProfileHeader
      profile={profile}
      contactStatus={status}
      actionLoading={false}
      isBlocked={false}
      {...handlers}
      {...extra}
    />,
  )
  return handlers
}

describe("ProfileHeader", () => {
  it("shows the display name and bio", () => {
    setup("none")
    expect(screen.getByRole("heading", { name: "Ada" })).toBeInTheDocument()
    expect(screen.getByText("Loves math")).toBeInTheDocument()
  })

  it("adds a contact when there is no relationship", () => {
    const h = setup("none")
    fireEvent.click(screen.getByRole("button", { name: /Add contact/ }))
    expect(h.onAddContact).toHaveBeenCalledOnce()
  })

  it("messages an existing contact", () => {
    const h = setup("contact")
    fireEvent.click(screen.getByRole("button", { name: "Message" }))
    expect(h.onStartDM).toHaveBeenCalledOnce()
  })

  it("accepts an incoming request", () => {
    const h = setup("incoming")
    fireEvent.click(screen.getByRole("button", { name: "Accept request" }))
    expect(h.onAccept).toHaveBeenCalledOnce()
  })

  it("shows a sent-request badge without an action button", () => {
    setup("sent")
    expect(screen.getByText("Request sent")).toBeInTheDocument()
  })

  it("hides the action controls while the status is loading", () => {
    setup("loading")
    expect(screen.queryByText("Block user")).toBeNull()
  })

  it("exposes block and report affordances", () => {
    const h = setup("none")
    fireEvent.click(screen.getByRole("button", { name: "Block user" }))
    fireEvent.click(screen.getByRole("button", { name: "Report" }))
    expect(h.onBlock).toHaveBeenCalledOnce()
    expect(h.onReport).toHaveBeenCalledOnce()
  })

  it("omits report when no handler is provided", () => {
    renderWithIntl(
      <ProfileHeader
        profile={profile}
        contactStatus="none"
        actionLoading={false}
        isBlocked={false}
        onAddContact={vi.fn()}
        onAccept={vi.fn()}
        onStartDM={vi.fn()}
        onBlock={vi.fn()}
        onUnblock={vi.fn()}
      />,
    )
    expect(screen.queryByText("Report")).toBeNull()
  })

  it("shows the blocked banner and unblocks", () => {
    const h = setup("contact", { isBlocked: true })
    expect(screen.getByText("You have blocked this user")).toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "Unblock" }))
    expect(h.onUnblock).toHaveBeenCalledOnce()
  })
})
