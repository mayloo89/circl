import { describe, it, expect, beforeEach } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { SidebarProvider, useSidebar } from "@/contexts/SidebarContext"

function Probe() {
  const { collapsed, toggle } = useSidebar()
  return (
    <button onClick={toggle} data-collapsed={String(collapsed)}>
      toggle
    </button>
  )
}

describe("SidebarContext", () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it("starts expanded by default", () => {
    render(
      <SidebarProvider>
        <Probe />
      </SidebarProvider>,
    )
    expect(screen.getByRole("button")).toHaveAttribute("data-collapsed", "false")
  })

  it("restores the persisted collapsed state", () => {
    localStorage.setItem("sidebar-collapsed", "true")
    render(
      <SidebarProvider>
        <Probe />
      </SidebarProvider>,
    )
    expect(screen.getByRole("button")).toHaveAttribute("data-collapsed", "true")
  })

  it("toggle flips and persists the state", () => {
    render(
      <SidebarProvider>
        <Probe />
      </SidebarProvider>,
    )
    fireEvent.click(screen.getByRole("button"))
    expect(screen.getByRole("button")).toHaveAttribute("data-collapsed", "true")
    expect(localStorage.getItem("sidebar-collapsed")).toBe("true")

    fireEvent.click(screen.getByRole("button"))
    expect(screen.getByRole("button")).toHaveAttribute("data-collapsed", "false")
    expect(localStorage.getItem("sidebar-collapsed")).toBe("false")
  })
})
