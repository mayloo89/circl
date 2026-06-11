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

import RecentConversationsWidget from "@/components/home/RecentConversationsWidget"

const API = "http://localhost:8080"

function room(overrides: Record<string, unknown> = {}) {
  return {
    id: "r1",
    type: "dm",
    name: "Bea",
    peer_avatar_url: "",
    last_message: { content: "see you there", created_at: new Date().toISOString() },
    unread_count: 0,
    ...overrides,
  }
}

describe("RecentConversationsWidget", () => {
  it("renders recent rooms with their last message", async () => {
    server.use(http.get(`${API}/chat/rooms`, () => HttpResponse.json([room()])))
    renderWithIntl(<RecentConversationsWidget />)
    expect(await screen.findByText("Bea")).toBeInTheDocument()
    expect(screen.getByText("see you there")).toBeInTheDocument()
    expect(screen.getByText("Bea").closest("a")).toHaveAttribute("href", "/en/chat/r1")
  })

  it("caps the unread badge at 9+", async () => {
    server.use(
      http.get(`${API}/chat/rooms`, () =>
        HttpResponse.json([room({ unread_count: 12 })]),
      ),
    )
    renderWithIntl(<RecentConversationsWidget />)
    expect(await screen.findByText("9+")).toBeInTheDocument()
  })

  it("shows the empty state when there are no rooms", async () => {
    server.use(http.get(`${API}/chat/rooms`, () => HttpResponse.json([])))
    renderWithIntl(<RecentConversationsWidget />)
    expect(await screen.findByText("No conversations yet")).toBeInTheDocument()
  })

  it("recovers from a failed fetch via retry", async () => {
    server.use(http.get(`${API}/chat/rooms`, () => HttpResponse.json({}, { status: 500 })))
    renderWithIntl(<RecentConversationsWidget />)
    expect(await screen.findByText("Couldn’t load")).toBeInTheDocument()

    server.use(http.get(`${API}/chat/rooms`, () => HttpResponse.json([room()])))
    fireEvent.click(screen.getByRole("button", { name: "Retry" }))
    expect(await screen.findByText("Bea")).toBeInTheDocument()
  })
})
