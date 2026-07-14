import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"
import ContactCard, { type ContactCardVariant } from "@/components/contacts/ContactCard"

function setup(variant: ContactCardVariant, extra = {}) {
  const onNavigate = vi.fn()
  const onPrimary = vi.fn()
  const onSecondary = vi.fn()
  renderWithIntl(
    <ul>
      <ContactCard
        userId="u1"
        email="ada@example.com"
        displayName="Ada"
        avatarUrl=""
        variant={variant}
        onNavigate={onNavigate}
        onPrimary={onPrimary}
        onSecondary={onSecondary}
        {...extra}
      />
    </ul>,
  )
  return { onNavigate, onPrimary, onSecondary }
}

describe("ContactCard", () => {
  it("renders a search result with an Add action and no navigate button", () => {
    const { onPrimary } = setup("search-result")
    expect(screen.getByText("Ada")).toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "Add" }))
    expect(onPrimary).toHaveBeenCalledOnce()
  })

  it("renders pending requests with accept and decline", () => {
    const { onPrimary, onSecondary } = setup("pending")
    fireEvent.click(screen.getByRole("button", { name: "Accept" }))
    fireEvent.click(screen.getByRole("button", { name: "Decline" }))
    expect(onPrimary).toHaveBeenCalledOnce()
    expect(onSecondary).toHaveBeenCalledOnce()
  })

  it("renders a sent request with a cancel action", () => {
    const { onSecondary } = setup("sent")
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }))
    expect(onSecondary).toHaveBeenCalledOnce()
  })

  it("renders a contact with message, remove, presence and navigation", () => {
    const { onPrimary, onSecondary, onNavigate } = setup("contact", { online: true })
    fireEvent.click(screen.getByRole("button", { name: "Message" }))
    fireEvent.click(screen.getByRole("button", { name: "Remove" }))
    fireEvent.click(screen.getByText("Ada"))
    expect(onPrimary).toHaveBeenCalledOnce()
    expect(onSecondary).toHaveBeenCalledOnce()
    expect(onNavigate).toHaveBeenCalledOnce()
  })

  it("falls back to the email when no display name is set", () => {
    renderWithIntl(
      <ul>
        <ContactCard userId="u1" email="x@y.z" displayName="" avatarUrl="" variant="search-result" />
      </ul>,
    )
    expect(screen.getByText("x@y.z")).toBeInTheDocument()
  })
})
