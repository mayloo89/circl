import { describe, it, expect, vi } from "vitest"
import { screen, fireEvent } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import { renderWithIntl } from "@/test/renderWithIntl"

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { accessToken: "tok" }, status: "authenticated" }),
}))

vi.mock("@/i18n/navigation", () => ({
  Link: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={`/en${href}`} {...rest}>
      {children}
    </a>
  ),
}))

import NearbyProfilesWidget from "@/components/home/NearbyProfilesWidget"

const API = "http://localhost:8080"

function profile(overrides: Record<string, unknown> = {}) {
  return {
    user_id: "u2",
    username: "bea",
    display_name: "Bea",
    avatar_url: "",
    distance_km: 2.4,
    location_text: "Palermo",
    ...overrides,
  }
}

describe("NearbyProfilesWidget", () => {
  it("renders nearby profiles with rounded distance, linking to their pages", async () => {
    server.use(
      http.get(`${API}/profiles/browse`, () =>
        HttpResponse.json({ profiles: [profile()] }),
      ),
    )
    renderWithIntl(<NearbyProfilesWidget />)
    expect(await screen.findByText("Bea")).toBeInTheDocument()
    expect(screen.getByText("2 km")).toBeInTheDocument()
    expect(screen.getByText("Bea").closest("a")).toHaveAttribute("href", "/en/profile/bea")
  })

  it("falls back to the location text when distance is hidden", async () => {
    server.use(
      http.get(`${API}/profiles/browse`, () =>
        HttpResponse.json({ profiles: [profile({ distance_km: null })] }),
      ),
    )
    renderWithIntl(<NearbyProfilesWidget />)
    expect(await screen.findByText("Palermo")).toBeInTheDocument()
  })

  it("shows the sub-kilometre label for very close profiles", async () => {
    server.use(
      http.get(`${API}/profiles/browse`, () =>
        HttpResponse.json({ profiles: [profile({ distance_km: 0.3 })] }),
      ),
    )
    renderWithIntl(<NearbyProfilesWidget />)
    expect(await screen.findByText("< 1 km")).toBeInTheDocument()
  })

  it("shows the empty state with a set-location link", async () => {
    server.use(http.get(`${API}/profiles/browse`, () => HttpResponse.json({ profiles: [] })))
    renderWithIntl(<NearbyProfilesWidget />)
    expect(await screen.findByText("No nearby profiles with a location set.")).toBeInTheDocument()
  })

  it("recovers from a failed fetch via retry", async () => {
    server.use(http.get(`${API}/profiles/browse`, () => HttpResponse.json({}, { status: 500 })))
    renderWithIntl(<NearbyProfilesWidget />)
    expect(await screen.findByText("Couldn’t load")).toBeInTheDocument()

    server.use(
      http.get(`${API}/profiles/browse`, () =>
        HttpResponse.json({ profiles: [profile()] }),
      ),
    )
    fireEvent.click(screen.getByRole("button", { name: "Retry" }))
    expect(await screen.findByText("Bea")).toBeInTheDocument()
  })
})
