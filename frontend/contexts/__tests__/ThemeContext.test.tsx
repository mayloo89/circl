import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { ThemeProvider, useTheme } from "@/contexts/ThemeContext"

function Probe() {
  const { theme, toggle } = useTheme()
  return (
    <button onClick={toggle} data-theme={theme}>
      toggle
    </button>
  )
}

function stubMatchMedia(prefersDark: boolean) {
  vi.stubGlobal(
    "matchMedia",
    vi.fn().mockReturnValue({
      matches: prefersDark,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }),
  )
}

describe("ThemeContext", () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove("dark")
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("falls back to the OS preference when nothing is stored", () => {
    stubMatchMedia(true)
    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    )
    expect(screen.getByRole("button")).toHaveAttribute("data-theme", "dark")
    expect(document.documentElement.classList.contains("dark")).toBe(true)
  })

  it("prefers the stored choice over the OS preference", () => {
    stubMatchMedia(true)
    localStorage.setItem("theme", "light")
    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    )
    expect(screen.getByRole("button")).toHaveAttribute("data-theme", "light")
    expect(document.documentElement.classList.contains("dark")).toBe(false)
  })

  it("toggle flips the theme and persists it to localStorage and the cookie", () => {
    stubMatchMedia(false)
    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    )
    expect(screen.getByRole("button")).toHaveAttribute("data-theme", "light")

    fireEvent.click(screen.getByRole("button"))
    expect(screen.getByRole("button")).toHaveAttribute("data-theme", "dark")
    expect(localStorage.getItem("theme")).toBe("dark")
    expect(document.cookie).toContain("theme=dark")
    expect(document.documentElement.classList.contains("dark")).toBe(true)

    fireEvent.click(screen.getByRole("button"))
    expect(localStorage.getItem("theme")).toBe("light")
    expect(document.documentElement.classList.contains("dark")).toBe(false)
  })
})
