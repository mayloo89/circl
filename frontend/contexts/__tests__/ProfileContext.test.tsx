import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, waitFor, act } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"

let sessionState: { data: { accessToken: string } | null; status: string } = {
  data: null,
  status: "unauthenticated",
}

vi.mock("next-auth/react", () => ({
  useSession: () => sessionState,
}))

import { ProfileProvider, useProfileContext } from "@/contexts/ProfileContext"

const API = "http://localhost:8080"

function Probe() {
  const { profile, refresh } = useProfileContext()
  return (
    <div>
      <span data-testid="name">{profile?.display_name ?? "none"}</span>
      <button onClick={() => void refresh()}>refresh</button>
    </div>
  )
}

const profilePayload = {
  username: "alice",
  display_name: "Alice",
  avatar_url: "",
  bio: "hi",
  interests: ["music"],
  location_text: "BA",
  onboarded_at: null,
}

describe("ProfileContext", () => {
  beforeEach(() => {
    sessionState = { data: null, status: "unauthenticated" }
  })

  it("stays empty while unauthenticated", () => {
    render(
      <ProfileProvider>
        <Probe />
      </ProfileProvider>,
    )
    expect(screen.getByTestId("name")).toHaveTextContent("none")
  })

  it("loads the profile once authenticated", async () => {
    sessionState = { data: { accessToken: "tok" }, status: "authenticated" }
    server.use(
      http.get(`${API}/profiles/me`, ({ request }) => {
        expect(request.headers.get("Authorization")).toBe("Bearer tok")
        return HttpResponse.json(profilePayload)
      }),
    )
    render(
      <ProfileProvider>
        <Probe />
      </ProfileProvider>,
    )
    await waitFor(() => expect(screen.getByTestId("name")).toHaveTextContent("Alice"))
  })

  it("refresh re-fetches the profile", async () => {
    sessionState = { data: { accessToken: "tok" }, status: "authenticated" }
    let displayName = "Alice"
    server.use(
      http.get(`${API}/profiles/me`, () =>
        HttpResponse.json({ ...profilePayload, display_name: displayName }),
      ),
    )
    render(
      <ProfileProvider>
        <Probe />
      </ProfileProvider>,
    )
    await waitFor(() => expect(screen.getByTestId("name")).toHaveTextContent("Alice"))

    displayName = "Alicia"
    await act(async () => {
      screen.getByRole("button", { name: "refresh" }).click()
    })
    await waitFor(() => expect(screen.getByTestId("name")).toHaveTextContent("Alicia"))
  })

  it("keeps the previous profile when the fetch fails", async () => {
    sessionState = { data: { accessToken: "tok" }, status: "authenticated" }
    let fail = false
    server.use(
      http.get(`${API}/profiles/me`, () =>
        fail
          ? HttpResponse.json({ error: "boom" }, { status: 500 })
          : HttpResponse.json(profilePayload),
      ),
    )
    render(
      <ProfileProvider>
        <Probe />
      </ProfileProvider>,
    )
    await waitFor(() => expect(screen.getByTestId("name")).toHaveTextContent("Alice"))

    fail = true
    await act(async () => {
      screen.getByRole("button", { name: "refresh" }).click()
    })
    expect(screen.getByTestId("name")).toHaveTextContent("Alice")
  })
})
