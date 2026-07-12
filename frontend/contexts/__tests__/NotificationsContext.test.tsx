import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, act, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { server } from "@/test/msw-server"
import type { ContactEvent } from "@/hooks/useNotifications"

const API = "http://localhost:8080"

const hoisted = vi.hoisted(() => ({ emit: null as null | ((e: ContactEvent) => void) }))

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { accessToken: "tok" }, status: "authenticated" }),
}))
vi.mock("@/hooks/useHeartbeat", () => ({ useHeartbeat: () => {} }))
vi.mock("@/hooks/useNotifications", () => ({
  useNotifications: (_token: string, onEvent: (e: ContactEvent) => void) => {
    hoisted.emit = onEvent
  },
}))

import { NotificationsProvider, useNotificationsContext } from "@/contexts/NotificationsContext"

const received: ContactEvent[] = []

function Consumer() {
  const { pendingCount, unreadChatCount, clearChatBadge, subscribe } = useNotificationsContext()
  return (
    <div>
      <span data-testid="pending">{pendingCount}</span>
      <span data-testid="unread">{unreadChatCount}</span>
      <button onClick={clearChatBadge}>clear</button>
      <button onClick={() => subscribe((e) => received.push(e))}>subscribe</button>
    </div>
  )
}

function emit(e: ContactEvent) {
  act(() => hoisted.emit?.(e))
}

describe("NotificationsContext", () => {
  beforeEach(() => {
    received.length = 0
    hoisted.emit = null
    server.use(
      http.get(`${API}/contacts/pending`, () => HttpResponse.json([{ id: "p1" }, { id: "p2" }])),
      http.get(`${API}/chat/rooms`, () =>
        HttpResponse.json([{ unread_count: 2 }, { unread_count: 3 }]),
      ),
    )
  })

  it("loads the initial pending and unread counts", async () => {
    render(
      <NotificationsProvider>
        <Consumer />
      </NotificationsProvider>,
    )
    await waitFor(() => expect(screen.getByTestId("pending")).toHaveTextContent("2"))
    expect(screen.getByTestId("unread")).toHaveTextContent("5")
  })

  it("increments the badges on incoming SSE events", async () => {
    render(
      <NotificationsProvider>
        <Consumer />
      </NotificationsProvider>,
    )
    await waitFor(() => expect(screen.getByTestId("pending")).toHaveTextContent("2"))

    emit({ type: "contact_request" } as ContactEvent)
    expect(screen.getByTestId("pending")).toHaveTextContent("3")

    emit({ type: "new_message" } as ContactEvent)
    expect(screen.getByTestId("unread")).toHaveTextContent("6")
  })

  it("clears the chat badge on demand", async () => {
    render(
      <NotificationsProvider>
        <Consumer />
      </NotificationsProvider>,
    )
    await waitFor(() => expect(screen.getByTestId("unread")).toHaveTextContent("5"))
    fireEvent.click(screen.getByRole("button", { name: "clear" }))
    expect(screen.getByTestId("unread")).toHaveTextContent("0")
  })

  it("refreshes the pending count after a contact is removed", async () => {
    render(
      <NotificationsProvider>
        <Consumer />
      </NotificationsProvider>,
    )
    await waitFor(() => expect(screen.getByTestId("pending")).toHaveTextContent("2"))

    server.use(http.get(`${API}/contacts/pending`, () => HttpResponse.json([{ id: "p1" }])))
    emit({ type: "contact_removed" } as ContactEvent)
    await waitFor(() => expect(screen.getByTestId("pending")).toHaveTextContent("1"))
  })

  it("fans events out to subscribers", async () => {
    render(
      <NotificationsProvider>
        <Consumer />
      </NotificationsProvider>,
    )
    await waitFor(() => expect(screen.getByTestId("pending")).toHaveTextContent("2"))
    fireEvent.click(screen.getByRole("button", { name: "subscribe" }))
    emit({ type: "new_message" } as ContactEvent)
    expect(received).toHaveLength(1)
    expect(received[0].type).toBe("new_message")
  })

  it("provides safe defaults outside the provider", () => {
    function Bare() {
      const { pendingCount, subscribe } = useNotificationsContext()
      const unsub = subscribe(() => {})
      unsub()
      return <span>{pendingCount}</span>
    }
    render(<Bare />)
    expect(screen.getByText("0")).toBeInTheDocument()
  })
})
