import { describe, it, expect, vi, beforeEach } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"

const API = "http://localhost:8080"

const enable = vi.fn()
let push = { permission: "default" as string, supported: true, enable }

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { accessToken: "tok" }, status: "authenticated" }),
}))
vi.mock("@/contexts/PushContext", () => ({ usePushContext: () => push }))

import PushPrompt from "@/components/PushPrompt"

describe("PushPrompt", () => {
  beforeEach(() => {
    push = { permission: "default", supported: true, enable }
    enable.mockClear()
    server.use(http.get(`${API}/chat/rooms`, () => HttpResponse.json([{ id: "r1" }])))
  })

  it("appears once the user has active rooms and enables push", async () => {
    renderWithIntl(<PushPrompt />)
    fireEvent.click(await screen.findByText("Enable"))
    expect(enable).toHaveBeenCalledOnce()
  })

  it("dismisses on the close control", async () => {
    renderWithIntl(<PushPrompt />)
    await screen.findByText("Enable")
    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }))
    expect(screen.queryByText("Enable")).toBeNull()
  })

  it("stays hidden when push is already granted", async () => {
    push.permission = "granted"
    renderWithIntl(<PushPrompt />)
    // give the rooms fetch a chance to resolve
    await new Promise((r) => setTimeout(r, 0))
    expect(screen.queryByText("Enable")).toBeNull()
  })

  it("stays hidden when push is unsupported", async () => {
    push.supported = false
    renderWithIntl(<PushPrompt />)
    await new Promise((r) => setTimeout(r, 0))
    expect(screen.queryByText("Enable")).toBeNull()
  })
})
