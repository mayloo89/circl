import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"
import Lightbox from "@/components/chat/Lightbox"

describe("Lightbox", () => {
  it("renders an image and closes on the close button", () => {
    const onClose = vi.fn()
    renderWithIntl(<Lightbox url="/pic.jpg" type="image" onClose={onClose} />)
    const img = screen.getByAltText("Full size image")
    expect(img).toHaveAttribute("src", "/pic.jpg")
    fireEvent.click(screen.getByRole("button", { name: "Close" }))
    expect(onClose).toHaveBeenCalled()
  })

  it("renders a video element for video content", () => {
    const { container } = renderWithIntl(
      <Lightbox url="/clip.mp4" type="video" onClose={vi.fn()} />,
    )
    const video = container.querySelector("video")
    expect(video).not.toBeNull()
    expect(video).toHaveAttribute("src", "/clip.mp4")
  })

  it("does not close when the media itself is clicked", () => {
    const onClose = vi.fn()
    renderWithIntl(<Lightbox url="/pic.jpg" type="image" onClose={onClose} />)
    fireEvent.click(screen.getByAltText("Full size image"))
    expect(onClose).not.toHaveBeenCalled()
  })
})
