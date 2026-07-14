import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"

const API = "http://localhost:8080"

let session: Record<string, unknown> | null = null
const signOut = vi.fn(() => Promise.resolve())

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: session, status: "authenticated" }),
  signOut: () => signOut(),
}))

import SignOutButton from "@/components/SignOutButton"

describe("SignOutButton", () => {
  beforeEach(() => {
    session = null
    signOut.mockClear()
  })

  it("signs out immediately when there is no session token", async () => {
    render(<SignOutButton />)
    fireEvent.click(screen.getByRole("button", { name: "Sign Out" }))
    await waitFor(() => expect(signOut).toHaveBeenCalledOnce())
  })

  it("revokes presence and refresh token before signing out", async () => {
    session = { accessToken: "tok", refreshToken: "ref" }
    let heartbeatDeleted = false
    let logoutCalled = false
    server.use(
      http.delete(`${API}/presence/heartbeat`, () => {
        heartbeatDeleted = true
        return new HttpResponse(null, { status: 204 })
      }),
      http.post(`${API}/auth/logout`, () => {
        logoutCalled = true
        return HttpResponse.json({})
      }),
    )
    render(<SignOutButton />)
    fireEvent.click(screen.getByRole("button", { name: "Sign Out" }))
    await waitFor(() => expect(signOut).toHaveBeenCalledOnce())
    expect(heartbeatDeleted).toBe(true)
    expect(logoutCalled).toBe(true)
  })

  it("still signs out when the revoke requests fail", async () => {
    session = { accessToken: "tok", refreshToken: "ref" }
    server.use(
      http.delete(`${API}/presence/heartbeat`, () => HttpResponse.error()),
      http.post(`${API}/auth/logout`, () => HttpResponse.error()),
    )
    render(<SignOutButton />)
    fireEvent.click(screen.getByRole("button", { name: "Sign Out" }))
    await waitFor(() => expect(signOut).toHaveBeenCalledOnce())
  })
})
