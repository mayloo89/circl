import { describe, it, expect } from "vitest"
import { render, screen } from "@testing-library/react"
import TypingIndicator from "@/components/chat/TypingIndicator"
import DateSeparator from "@/components/chat/DateSeparator"

describe("TypingIndicator", () => {
  it("renders nothing when nobody is typing", () => {
    const { container } = render(<TypingIndicator typers={[]} />)
    expect(container).toBeEmptyDOMElement()
  })

  it("uses singular phrasing for one typer", () => {
    render(<TypingIndicator typers={["Bea"]} />)
    expect(screen.getByText(/Bea is typing/)).toBeInTheDocument()
  })

  it("joins multiple typers with plural phrasing", () => {
    render(<TypingIndicator typers={["Bea", "Carla"]} />)
    expect(screen.getByText(/Bea, Carla are typing/)).toBeInTheDocument()
  })
})

describe("DateSeparator", () => {
  it("renders the label", () => {
    render(<DateSeparator label="Today" />)
    expect(screen.getByText("Today")).toBeInTheDocument()
  })
})
