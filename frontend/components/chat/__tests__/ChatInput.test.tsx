import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"
import ChatInput from "@/components/chat/ChatInput"

const baseProps = {
  connected: true,
  uploading: false,
  ephemeral: "off" as const,
  onEphemeralChange: vi.fn(),
  onSend: vi.fn(),
  onAttach: vi.fn(),
  onTyping: vi.fn(),
}

describe("ChatInput", () => {
  it("sends trimmed content on Enter and clears the input", () => {
    const onSend = vi.fn()
    renderWithIntl(<ChatInput {...baseProps} onSend={onSend} />)
    const input = screen.getByLabelText("Message…")
    fireEvent.change(input, { target: { value: "  hola  " } })
    fireEvent.keyDown(input, { key: "Enter" })
    expect(onSend).toHaveBeenCalledWith("hola")
    expect(input).toHaveValue("")
  })

  it("does not send on Shift+Enter", () => {
    const onSend = vi.fn()
    renderWithIntl(<ChatInput {...baseProps} onSend={onSend} />)
    const input = screen.getByLabelText("Message…")
    fireEvent.change(input, { target: { value: "line one" } })
    fireEvent.keyDown(input, { key: "Enter", shiftKey: true })
    expect(onSend).not.toHaveBeenCalled()
  })

  it("never sends whitespace-only content", () => {
    const onSend = vi.fn()
    renderWithIntl(<ChatInput {...baseProps} onSend={onSend} />)
    const input = screen.getByLabelText("Message…")
    fireEvent.change(input, { target: { value: "   " } })
    fireEvent.keyDown(input, { key: "Enter" })
    expect(onSend).not.toHaveBeenCalled()
  })

  it("disables the send button while disconnected", () => {
    renderWithIntl(<ChatInput {...baseProps} connected={false} />)
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled()
  })

  it("shows the contact-sharing warning when enabled", () => {
    renderWithIntl(<ChatInput {...baseProps} showContactWarning />)
    expect(
      screen.getByText("Sharing contact info outside accepted contacts is not allowed."),
    ).toBeInTheDocument()
  })

  it("hides the contact-sharing warning by default", () => {
    renderWithIntl(<ChatInput {...baseProps} />)
    expect(
      screen.queryByText("Sharing contact info outside accepted contacts is not allowed."),
    ).not.toBeInTheDocument()
  })

  it("throttles typing notifications", () => {
    const onTyping = vi.fn()
    renderWithIntl(<ChatInput {...baseProps} onTyping={onTyping} />)
    const input = screen.getByLabelText("Message…")
    fireEvent.change(input, { target: { value: "h" } })
    fireEvent.change(input, { target: { value: "ho" } })
    fireEvent.change(input, { target: { value: "hol" } })
    expect(onTyping).toHaveBeenCalledOnce()
  })

  it("opens the ephemeral menu and selects a mode", () => {
    const onEphemeralChange = vi.fn()
    renderWithIntl(<ChatInput {...baseProps} onEphemeralChange={onEphemeralChange} />)
    fireEvent.click(screen.getByRole("button", { name: "Ephemeral message" }))
    fireEvent.click(screen.getByRole("menuitem", { name: "View once" }))
    expect(onEphemeralChange).toHaveBeenCalledWith("view_once")
  })

  it("shows the active ephemeral mode banner", () => {
    renderWithIntl(<ChatInput {...baseProps} ephemeral="view_once" />)
    // Banner above the input + menu trigger both reference the mode.
    expect(screen.getByText("View once")).toBeInTheDocument()
  })

  it("hides the ephemeral button when disabled (channels)", () => {
    renderWithIntl(<ChatInput {...baseProps} disableEphemeral />)
    expect(screen.queryByRole("button", { name: "Ephemeral message" })).not.toBeInTheDocument()
  })

  it("passes a picked file to onAttach and resets the input", () => {
    const onAttach = vi.fn()
    const { container } = renderWithIntl(<ChatInput {...baseProps} onAttach={onAttach} />)
    const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement
    const file = new File(["x"], "pic.jpg", { type: "image/jpeg" })
    fireEvent.change(fileInput, { target: { files: [file] } })
    expect(onAttach).toHaveBeenCalledWith(file)
    expect(fileInput.value).toBe("")
  })

  it("hides the attach button when attachments are disabled", () => {
    renderWithIntl(<ChatInput {...baseProps} disableAttach />)
    expect(screen.queryByRole("button", { name: "Attach file" })).not.toBeInTheDocument()
  })

  it("offers an attach menu with album sharing when onShareAlbum is provided", () => {
    const onShareAlbum = vi.fn()
    renderWithIntl(<ChatInput {...baseProps} onShareAlbum={onShareAlbum} />)
    fireEvent.click(screen.getByRole("button", { name: "Attach file" }))
    fireEvent.click(screen.getByRole("menuitem", { name: "Share a private album" }))
    expect(onShareAlbum).toHaveBeenCalledOnce()
  })
})
