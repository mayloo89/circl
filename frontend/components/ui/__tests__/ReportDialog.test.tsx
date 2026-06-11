import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { renderWithIntl } from "@/test/renderWithIntl"
import ReportDialog from "@/components/ui/ReportDialog"

const baseProps = {
  open: true,
  onSubmit: vi.fn(),
  onCancel: vi.fn(),
}

describe("ReportDialog", () => {
  it("renders nothing when closed", () => {
    renderWithIntl(<ReportDialog {...baseProps} open={false} />)
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument()
  })

  it("lists every report reason", () => {
    renderWithIntl(<ReportDialog {...baseProps} />)
    const select = screen.getByLabelText(/reason/i) as HTMLSelectElement
    // placeholder + 8 reasons
    expect(select.options).toHaveLength(9)
    const values = Array.from(select.options).map((o) => o.value)
    expect(values).toContain("csam")
    expect(values).toContain("non_consensual_intimate_images")
    expect(values).toContain("digital_gender_violence")
    expect(values).toContain("harassment")
  })

  it("submits the chosen reason and description", () => {
    const onSubmit = vi.fn()
    renderWithIntl(<ReportDialog {...baseProps} onSubmit={onSubmit} />)
    fireEvent.change(screen.getByLabelText(/reason/i), { target: { value: "harassment" } })
    fireEvent.change(screen.getByLabelText(/description/i), {
      target: { value: "sent threats" },
    })
    fireEvent.submit(screen.getByLabelText(/reason/i).closest("form")!)
    expect(onSubmit).toHaveBeenCalledWith("harassment", "sent threats")
  })

  it("shows a submit error when provided", () => {
    renderWithIntl(<ReportDialog {...baseProps} error="Rate limit exceeded" />)
    expect(screen.getByText("Rate limit exceeded")).toBeInTheDocument()
  })

  it("cancels via the cancel button", () => {
    const onCancel = vi.fn()
    renderWithIntl(<ReportDialog {...baseProps} onCancel={onCancel} />)
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }))
    expect(onCancel).toHaveBeenCalledOnce()
  })
})
