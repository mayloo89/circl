import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import Button from "@/components/ui/Button"

describe("Button", () => {
  it("renders children", () => {
    render(<Button>Click me</Button>)
    expect(screen.getByRole("button", { name: "Click me" })).toBeInTheDocument()
  })

  it("calls onClick when clicked", async () => {
    const onClick = vi.fn()
    render(<Button onClick={onClick}>Go</Button>)
    await userEvent.click(screen.getByRole("button"))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it("is disabled when disabled prop is true", () => {
    render(<Button disabled>Save</Button>)
    expect(screen.getByRole("button")).toBeDisabled()
  })

  it("shows a spinner and is disabled when loading", () => {
    render(<Button loading>Saving</Button>)
    const btn = screen.getByRole("button")
    expect(btn).toBeDisabled()
    // spinner svg is present (aria-hidden, so query by DOM)
    expect(btn.querySelector("svg")).toBeInTheDocument()
  })

  it("does not fire onClick when disabled", async () => {
    const onClick = vi.fn()
    render(<Button disabled onClick={onClick}>Click</Button>)
    await userEvent.click(screen.getByRole("button"))
    expect(onClick).not.toHaveBeenCalled()
  })

  it("applies primary variant classes by default", () => {
    const { container } = render(<Button>Primary</Button>)
    expect(container.firstChild).toHaveClass("bg-indigo-600")
  })

  it("applies danger variant classes", () => {
    const { container } = render(<Button variant="danger">Delete</Button>)
    expect(container.firstChild).toHaveClass("bg-red-900")
  })

  it("applies ghost variant classes", () => {
    const { container } = render(<Button variant="ghost">Back</Button>)
    expect(container.firstChild).toHaveClass("text-gray-400")
  })

  it("applies rounded-full when pill is true", () => {
    const { container } = render(<Button pill>Round</Button>)
    expect(container.firstChild).toHaveClass("rounded-full")
  })

  it("applies sm size classes", () => {
    const { container } = render(<Button size="sm">Small</Button>)
    expect(container.firstChild).toHaveClass("text-xs")
  })

  it("passes extra props to the button element", () => {
    render(<Button type="submit">Submit</Button>)
    expect(screen.getByRole("button")).toHaveAttribute("type", "submit")
  })
})
