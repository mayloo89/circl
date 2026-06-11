import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"

vi.mock("@/i18n/navigation", () => ({
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
}))

import MessageBubble from "@/components/chat/MessageBubble"
import type { HistoryMessage } from "@/types/chat"

function message(overrides: Partial<HistoryMessage> = {}): HistoryMessage {
  return {
    id: "m1",
    room_id: "r1",
    sender_id: "u2",
    sender_name: "Bea",
    sender_avatar_url: "",
    type: "text",
    content: "hola",
    view_once: false,
    created_at: "2026-06-10T12:00:00Z",
    ...overrides,
  }
}

const baseProps = {
  isOwn: false,
  firstInGroup: true,
  lastInGroup: true,
  isLive: false,
  isTombstone: false,
  isLastSeenOwn: false,
  now: Date.parse("2026-06-10T12:05:00Z"),
  onViewOnce: vi.fn(),
  onOpenMedia: vi.fn(),
}

describe("MessageBubble", () => {
  it("renders plain text with the sender name for peer messages", () => {
    renderWithIntl(<MessageBubble {...baseProps} msg={message()} />)
    expect(screen.getByText("hola")).toBeInTheDocument()
    expect(screen.getByText("Bea")).toBeInTheDocument()
  })

  it("hides the sender name on own messages", () => {
    renderWithIntl(<MessageBubble {...baseProps} isOwn msg={message()} />)
    expect(screen.queryByText("Bea")).not.toBeInTheDocument()
  })

  it("renders system messages as a centered status pill", () => {
    renderWithIntl(
      <MessageBubble {...baseProps} msg={message({ type: "system", content: "Contact info hidden" })} />,
    )
    const pill = screen.getByRole("status")
    expect(pill).toHaveTextContent("Contact info hidden")
  })

  it("replaces the redaction sentinel with the localized note and preserves surrounding text", () => {
    renderWithIntl(
      <MessageBubble
        {...baseProps}
        msg={message({ redacted: true, content: "write me at [contact hidden] tonight" })}
      />,
    )
    expect(screen.getByRole("note")).toHaveTextContent(
      "Contact information was removed from the previous message to protect privacy.",
    )
    expect(screen.getByText(/write me at/)).toBeInTheDocument()
    expect(screen.getByText(/tonight/)).toBeInTheDocument()
    expect(screen.queryByText(/\[contact hidden\]/)).not.toBeInTheDocument()
  })

  it("renders an expired tombstone", () => {
    renderWithIntl(<MessageBubble {...baseProps} isTombstone msg={message({ content: "" })} />)
    expect(screen.getByText("Message expired")).toBeInTheDocument()
  })

  it("renders a view-once tombstone with distinct copy", () => {
    renderWithIntl(
      <MessageBubble {...baseProps} isTombstone msg={message({ content: "", view_once: true })} />,
    )
    expect(screen.getByText("View-once message")).toBeInTheDocument()
  })

  it("hides unrevealed view-once text behind a tap-to-read button", () => {
    const onViewOnce = vi.fn()
    renderWithIntl(
      <MessageBubble
        {...baseProps}
        onViewOnce={onViewOnce}
        msg={message({ view_once: true, content: "secret" })}
      />,
    )
    expect(screen.queryByText("secret")).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole("button", { name: "Tap to read" }))
    expect(onViewOnce).toHaveBeenCalledWith("m1")
  })

  it("shows revealed view-once text once provided", () => {
    renderWithIntl(
      <MessageBubble
        {...baseProps}
        revealedText="secret"
        msg={message({ view_once: true, content: "" })}
      />,
    )
    expect(screen.getByText("secret")).toBeInTheDocument()
  })

  it("opens images through onOpenMedia", () => {
    const onOpenMedia = vi.fn()
    renderWithIntl(
      <MessageBubble
        {...baseProps}
        onOpenMedia={onOpenMedia}
        msg={message({ type: "image", content: "http://cdn/full.jpg", thumbnail_url: "http://cdn/thumb.jpg" })}
      />,
    )
    fireEvent.click(screen.getByRole("button", { name: "Shared image" }))
    expect(onOpenMedia).toHaveBeenCalledWith("http://cdn/full.jpg", "image")
  })

  it("opens videos through onOpenMedia", () => {
    const onOpenMedia = vi.fn()
    renderWithIntl(
      <MessageBubble
        {...baseProps}
        onOpenMedia={onOpenMedia}
        msg={message({ type: "video", content: "http://cdn/v.mp4" })}
      />,
    )
    fireEvent.click(screen.getByRole("button", { name: "Play video" }))
    expect(onOpenMedia).toHaveBeenCalledWith("http://cdn/v.mp4", "video")
  })

  it("marks the last own message seen", () => {
    renderWithIntl(<MessageBubble {...baseProps} isOwn isLastSeenOwn msg={message()} />)
    expect(screen.getByText("Seen")).toBeInTheDocument()
  })
})
