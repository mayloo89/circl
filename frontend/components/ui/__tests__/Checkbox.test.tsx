import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import Checkbox from "@/components/ui/Checkbox"

describe("Checkbox", () => {
  it("renders the label and toggles when clicked", async () => {
    const onChange = vi.fn()
    render(<Checkbox label="Accept terms" onChange={onChange} />)
    const input = screen.getByRole("checkbox", { name: /accept terms/i })
    expect(input).not.toBeChecked()
    await userEvent.click(input)
    expect(onChange).toHaveBeenCalled()
  })

  it("renders ReactNode label content (e.g. embedded links)", () => {
    render(
      <Checkbox
        label={
          <>
            Accept the <a href="https://example.com/terms">Terms</a>
          </>
        }
      />,
    )
    expect(screen.getByRole("link", { name: "Terms" })).toBeInTheDocument()
  })

  it("sets aria-invalid + aria-describedby and renders alert when error is set", () => {
    render(<Checkbox label="x" error="You must agree" />)
    const input = screen.getByRole("checkbox")
    expect(input).toHaveAttribute("aria-invalid", "true")
    const message = screen.getByRole("alert")
    expect(message).toHaveTextContent("You must agree")
    expect(input.getAttribute("aria-describedby")).toBe(message.id)
  })

  it("omits aria-invalid + aria-describedby when no error", () => {
    render(<Checkbox label="x" />)
    const input = screen.getByRole("checkbox")
    expect(input).not.toHaveAttribute("aria-invalid")
    expect(input).not.toHaveAttribute("aria-describedby")
  })

  it("disables the input and applies cursor-not-allowed when disabled", () => {
    render(<Checkbox label="x" disabled />)
    expect(screen.getByRole("checkbox")).toBeDisabled()
  })

  it("uses the supplied id for label association", () => {
    render(<Checkbox id="my-id" label="x" />)
    expect(screen.getByRole("checkbox")).toHaveAttribute("id", "my-id")
  })
})
