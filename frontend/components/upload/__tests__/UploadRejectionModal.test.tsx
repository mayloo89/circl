import { describe, expect, it, vi } from "vitest"
import { fireEvent, render, screen } from "@testing-library/react"
import { NextIntlClientProvider } from "next-intl"

// Stub the next-intl navigation Link to a plain anchor so we can render the
// component without dragging in the full next-intl + next/navigation runtime.
vi.mock("@/i18n/navigation", () => ({
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
}))

import UploadRejectionModal from "@/components/upload/UploadRejectionModal"

const messages = {
  uploadRejection: {
    title: "Image rejected",
    intro: "We couldn't upload your image because it doesn't meet community guidelines.",
    fallback: "Please try another image.",
    reasonLabel: "Reason",
    code_nsfw_detected: "Explicit content was detected.",
    code_hash_match: "This image matches material on our block list.",
    code_size_out_of_bounds: "The image size doesn't meet requirements.",
    code_aspect_ratio_out_of_bounds: "The image proportions are unusual.",
    code_unknown: "The image didn't pass automated moderation.",
    guidelinesLink: "Read community guidelines",
    dismiss: "Got it",
    pendingTitle: "Still reviewing your image",
    pendingBody: "Your image is taking a little longer than usual to review.",
  },
}

function renderWithIntl(ui: React.ReactElement) {
  return render(
    <NextIntlClientProvider locale="en" messages={messages}>
      {ui}
    </NextIntlClientProvider>,
  )
}

describe("UploadRejectionModal", () => {
  it("renders nothing when rejection is null", () => {
    renderWithIntl(<UploadRejectionModal rejection={null} onClose={vi.fn()} />)
    expect(screen.queryByText("Image rejected")).not.toBeInTheDocument()
  })

  it("renders the localized NSFW body when code is nsfw_detected", () => {
    renderWithIntl(
      <UploadRejectionModal
        rejection={{ code: "nsfw_detected", reason: "classifier flagged" }}
        onClose={vi.fn()}
      />,
    )
    expect(screen.getByText("Image rejected")).toBeInTheDocument()
    expect(screen.getByText("Explicit content was detected.")).toBeInTheDocument()
    // Backend reason is shown as fine print for admin debugging.
    expect(screen.getByText(/classifier flagged/)).toBeInTheDocument()
  })

  it("falls back to the unknown copy for codes the UI doesn't know about", () => {
    renderWithIntl(
      <UploadRejectionModal rejection={{ code: "something_new" }} onClose={vi.fn()} />,
    )
    expect(screen.getByText("The image didn't pass automated moderation.")).toBeInTheDocument()
  })

  it("calls onClose when the dismiss button is clicked", () => {
    const onClose = vi.fn()
    renderWithIntl(
      <UploadRejectionModal rejection={{ code: "hash_match" }} onClose={onClose} />,
    )
    fireEvent.click(screen.getByText("Got it"))
    expect(onClose).toHaveBeenCalledOnce()
  })

  it("renders the pending variant when pendingReview is set and there is no rejection", () => {
    renderWithIntl(
      <UploadRejectionModal rejection={null} pendingReview onClose={vi.fn()} />,
    )
    expect(screen.getByText("Still reviewing your image")).toBeInTheDocument()
    expect(screen.getByText(/taking a little longer/)).toBeInTheDocument()
    // Not framed as a rejection.
    expect(screen.queryByText("Image rejected")).not.toBeInTheDocument()
  })

  it("prefers the rejection over pendingReview when both are set", () => {
    renderWithIntl(
      <UploadRejectionModal
        rejection={{ code: "nsfw_detected" }}
        pendingReview
        onClose={vi.fn()}
      />,
    )
    expect(screen.getByText("Image rejected")).toBeInTheDocument()
    expect(screen.queryByText("Still reviewing your image")).not.toBeInTheDocument()
  })
})
