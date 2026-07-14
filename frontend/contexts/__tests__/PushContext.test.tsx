import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"

const push = { permission: "granted", supported: true, enable: vi.fn(), disable: vi.fn() }

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { accessToken: "tok" }, status: "authenticated" }),
}))
vi.mock("@/hooks/usePush", () => ({ usePush: () => push }))

import { PushProvider, usePushContext } from "@/contexts/PushContext"

function Consumer() {
  const { permission, supported } = usePushContext()
  return <span>{`${permission}:${String(supported)}`}</span>
}

describe("PushContext", () => {
  it("exposes the push hook value to consumers", () => {
    render(
      <PushProvider>
        <Consumer />
      </PushProvider>,
    )
    expect(screen.getByText("granted:true")).toBeInTheDocument()
  })

  it("throws when used outside the provider", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {})
    expect(() => render(<Consumer />)).toThrow(/usePushContext must be used inside PushProvider/)
    spy.mockRestore()
  })
})
